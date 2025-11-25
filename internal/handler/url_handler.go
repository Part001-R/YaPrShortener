// handler пакет. Секция с обработчиками контроллера HTTP.
// Содержит конструкторы.
package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/service/flags"
	gz "github.com/Part001-R/YaPrShortener/internal/service/gzip"
	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const (
	shorten = "shorten"
	follow  = "follow"
)

// Для передачи в Go асинхронной очистки БД.
type DeleteDB struct {
	Short string
	UUID  string
}

// Для хранения в памяти.
type ShortLongURL struct {
	ShorByLong  map[string]string
	LongByShort map[string]string
	mu          sync.RWMutex
}

// Для взаимодействия с БД.
type ShortLongDB struct {
	Ptr         *sql.DB
	mu          sync.RWMutex
	ChForDelete chan DeleteDB
	ChDoDelete  chan struct{}
}

// Сервис сокращения ссылок.
type ShortLong struct {
	List             *ShortLongURL
	DB               *ShortLongDB
	Observer         observer.Action
	BaseAddrShortURL string
	ServerAddr       string
	FileStoragePath  string
	Log              *zap.Logger
	wg               sync.WaitGroup // учёт активности обработчиков запросов и асинхронного удаления.
	stopping         bool           // true - признак, что сервис в процессе остановки.

}

// Для передачи содержимого файла в память.
type EventURL struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Для длинного представления URL.
type rxLongURL struct {
	URL string `json:"url"`
}

// Для короткого представления URL.
type txShortURL struct {
	Result string `json:"result"`
}

// Для приёма длинного представления URL и ID.
type rxLongURLBatch struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// Для передачи сокращённого URL с ID.
type txShortURLBatch struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// Для передачи длинного URL с ID.
type txShortURLOriginalURL struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type handlers interface {
	ShortURLFromLong(w http.ResponseWriter, r *http.Request)
	LongURLFromShort(w http.ResponseWriter, r *http.Request)
	ShortURLFromLongJSON(w http.ResponseWriter, r *http.Request)
	ShortURLFromLongBatch(w http.ResponseWriter, r *http.Request)
	UserURLs(w http.ResponseWriter, r *http.Request)
	DeleteUserURLs(w http.ResponseWriter, r *http.Request)
}

type systemAct interface {
	PingDB(w http.ResponseWriter, r *http.Request)
	WaitFinActions()
	SetFlagStopping()
	WGAdd()
	WGDone()
	IsFlagStopping() bool
}

type file interface {
	LoadFileURL() error
}

type middleware interface {
	Middleware(h http.Handler) http.Handler
	MiddlewareAudit(h http.Handler) http.Handler
}

// Основной интерфейс.
type Actions interface {
	middleware
	systemAct
	handlers
	file
}

// Реализация проверки кодировки и формирование длительности выполнения обработчика запроса.
func (sl *ShortLong) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w

		// Проверка поддерживает ли сервер запрашиваемую клиентом кодировку
		acceptEncoding := r.Header.Get("Accept-Encoding")
		found := false

		if acceptEncoding != "" {
			encodings := strings.Split(acceptEncoding, ",")

			for _, v := range encodings {
				encodingType := strings.TrimSpace(v)

				switch encodingType {
				case "gzip":
					cw := gz.NewCompressWriter(w)
					ow = cw
					defer func() {
						if err := cw.Close(); err != nil {
							sl.Log.Error("Ошибка при закрытии cw", zap.Error(err))
						}
					}()
					found = true
				case "identity":
					found = true
				}
			}

			if !found {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}

		// Проверка, как клиент закодировал переданные данные
		contentEncoding := r.Header.Get("Content-Encoding")
		found = false

		if contentEncoding != "" {
			encodings := strings.Split(contentEncoding, ",")

			for _, v := range encodings {
				encodingType := strings.TrimSpace(v)

				switch encodingType {
				case "gzip":
					cr, err := gz.NewCompressReader(r.Body)
					if err != nil {
						sl.Log.Error("Ошибка в NewCompressReader", zap.Error(err))
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
					defer func() {
						if err := cr.Close(); err != nil {
							sl.Log.Error("Ошибка при закрытии cr", zap.Error(err))
						}
					}()
					defer func() {
						if err := r.Body.Close(); err != nil {
							sl.Log.Error("Ошибка при закрытии r.Body", zap.Error(err))
						}
					}()

					r.Body = cr
					found = true
				case "identity":
					found = true
				}
			}

			if !found {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}

		// Запуск обработчика
		timeStart := time.Now()
		h.ServeHTTP(ow, r) // Здесь вызываем обработчик
		duration := time.Since(timeStart)

		// Вывод в лог сводной информации по запросу
		sl.Log.Info("принят HTTP запрос",
			zap.String("URI", r.RequestURI),
			zap.String("метод", r.Method),
			zap.Duration("время выполнения запроса", duration),
		)
	})
}

// Реализация передачи аудиторам сообщения.
func (sl *ShortLong) MiddlewareAudit(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Проверка, что сервер в состоянии остановки.
		if sl.stopping {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Сервер временно недоступен"))
			return
		}

		// Логика
		//
		body, err := io.ReadAll(r.Body)
		if err != nil {
			sl.Log.Error("Ошибка при чтении тела запроса", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Восстановление тела запроса
		defer r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Рекордер для записи ответа
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, r)

		// Проверка кода ответа для оповещения наблюдателей
		if recorder.Code < 400 {

			locationHeader := recorder.Header().Get("Location")
			processingObserver(body, r.RequestURI, sl, locationHeader)
		}

		// Передача ответа в исходный ResponseWriter
		for k, v := range recorder.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(recorder.Code)
		w.Write(recorder.Body.Bytes())
	})
}

// Обработчик формирования короткого представления из длинной (POST "/").
func (sl *ShortLong) ShortURLFromLong(w http.ResponseWriter, r *http.Request) {

	sl.List.mu.RLock()
	defer sl.List.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Вся логика обработчика.
	internalShortURLFromLong(sl.DB.Ptr, sl, w, r)
}

// Обработчик формирования группы коротких представлений из группы длинных представлений URL (POST "/api/shorten/batch").
func (sl *ShortLong) ShortURLFromLongBatch(w http.ResponseWriter, r *http.Request) {

	sl.DB.mu.RLock()
	defer sl.DB.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Вся логика обработчика
	internalShortURLFromLongBatch(sl.DB.Ptr, sl, w, r)
}

// Обработчик представления оригинального URL, по принятому сокращению (GET "/{id}").
func (sl *ShortLong) LongURLFromShort(w http.ResponseWriter, r *http.Request) {

	sl.List.mu.RLock()
	defer sl.List.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	internalLongURLFromShort(sl.DB.Ptr, sl, w, r)
}

// Обработчик формирования короткого представления по длинной. Формат JSON (POST "/api/shorten").
func (sl *ShortLong) ShortURLFromLongJSON(w http.ResponseWriter, r *http.Request) {

	sl.List.mu.RLock()
	defer sl.List.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Вся логика обработчика
	internalShortURLFromLongJSON(sl.DB.Ptr, sl, w, r)
}

// Обработчик проверки связи с БД (GET "/ping").
func (sl *ShortLong) PingDB(w http.ResponseWriter, r *http.Request) {

	sl.wg.Add(1)
	defer sl.wg.Done()

	// Пинг
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := sl.DB.Ptr.PingContext(ctx); err != nil {
		sl.Log.Error("Ошибка выполнения ping БД",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Обработчик представления длинных URL (GET "/api/user/urls").
func (sl *ShortLong) UserURLs(w http.ResponseWriter, r *http.Request) {

	sl.DB.mu.RLock()
	defer sl.DB.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	internalUserURLs(sl.DB.Ptr, sl, w, r)

}

// Обработчик указания пар для асинхронного удаления (DELETE "/api/user/urls").
func (sl *ShortLong) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {

	sl.DB.mu.RLock()
	defer sl.DB.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	internalDeleteUserURLs(sl.DB.Ptr, sl, w, r)
}

// Копирование накопленных данных их файла в память. Возвращается ошибка.
func (sl *ShortLong) LoadFileURL() error {

	sl.List.mu.RLock()
	defer sl.List.mu.RUnlock()

	sl.wg.Add(1)
	defer sl.wg.Done()

	// Проверка
	if sl.FileStoragePath == "" {
		return errors.New("принят пустой путь к файлу хранения")
	}
	if sl.List.ShorByLong == nil {
		return errors.New("нет указателя на ShortByLong")
	}
	if sl.List.LongByShort == nil {
		return errors.New("нет указателя на LongByShort")
	}

	// Файл
	file, err := os.OpenFile(sl.FileStoragePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла <%s>: %v", sl.FileStoragePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			sl.Log.Error("Ошибка при закрытии подключения к файлу",
				zap.Error(err),
			)
		}
	}()

	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("ошибка получения статуса файла: %v", err)
	}
	if fi.Size() == 0 {
		return nil
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла: %v", err)
	}

	// Мапы
	var events []EventURL
	if err := json.Unmarshal(data, &events); err != nil {
		return fmt.Errorf("ошибка Unmarshal: %v", err)
	}

	for _, ev := range events {
		sl.List.ShorByLong[ev.OriginalURL] = ev.ShortURL
		sl.List.LongByShort[ev.ShortURL] = ev.OriginalURL
	}

	return nil
}

// ---

// Ожидание завершения работы активных обработчиков.
func (sl *ShortLong) WaitFinActions() {

	sl.wg.Wait()
}

// Установка признака, что активен процесс остановки.
func (sl *ShortLong) SetFlagStopping() {

	sl.Log.Info("Установлен признак запуска процесса остановки")
	sl.stopping = true
}

// Проверка признака, что активен процесс остановки. Возвращается true - в процессе остановки.
func (sl *ShortLong) IsFlagStopping() bool {

	return sl.stopping
}

// Установка признака активности процесса асинхронного удаления.
func (sl *ShortLong) WGAdd() {

	sl.wg.Add(1)
}

// Сброс признака активности асинхронного удаления.
func (sl *ShortLong) WGDone() {

	sl.wg.Done()
}

// ---

// Объект хранения данных в памяти.
var shortenrMemory *ShortLongURL

// Для обеспечения единоразового выполняения инициализации конструктором.
var OnceMemory sync.Once

// Конструктор. Возвращает хранилище в памяти.
func NewShortenerMemory() *ShortLongURL {
	OnceMemory.Do(func() {
		shortenrMemory = &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		}
	})
	return shortenrMemory
}

// Сконфигурироанный экземпляр хранения в БД.
var shortenrDB *ShortLongDB

// Для обеспечения единоразового выполняения инициализации конструктором.
var OnceDB sync.Once

// Конструктор. Возвращает хранилище БД.
//
// Параметры:
//
//	db - указатель на БД.
func NewShortenerDB(db *sql.DB) *ShortLongDB {
	OnceDB.Do(func() {
		shortenrDB = &ShortLongDB{
			Ptr:         db,
			mu:          sync.RWMutex{},
			ChForDelete: make(chan DeleteDB),
			ChDoDelete:  make(chan struct{}),
		}
	})
	return shortenrDB
}

// Сконфигурироанный экземпляр сервиса.
var shortener *ShortLong

// Для обеспечения единоразового выполняения инициализации конструктором.
var OnceShortener sync.Once

// Конструктор. Возвращает интерфейс объекта.
//
// Параметры:
//
//	storage - хранилище пар соответствий ссылок.
//	db - указатель на БД.
//	fl - флаги.
//	os - наблюдатель.
//	log - логгер.
func NewShortener(storage *ShortLongURL, db *ShortLongDB, fl *flags.Config, os observer.Action, log *zap.Logger) Actions {
	OnceShortener.Do(func() {
		shortener = &ShortLong{
			List:             storage,
			DB:               db,
			Observer:         os,
			BaseAddrShortURL: fl.BaseAddrShortURL,
			ServerAddr:       fl.Port,
			FileStoragePath:  fl.FileStoragePath,
			Log:              log,
			wg:               sync.WaitGroup{},
			stopping:         false,
		}
	})

	return shortener
}

// Функция наполняет мапы новыми парами соответствий длинных и коротких ссылок. Возвращает короткую ссылку и ошибку.
//
// Параметры:
//
//	sByL - мапа, где в качестве ключа - длинный URL, значения - короткое представление.
//	lByS - мапа, где в качестве ключа - короткое представление, значения - длинный URL.
//	longURL - исходное, длинное значение URL.
func fillListShortByLong(sByL, lByS map[string]string, longURL string) (string, error) {

	var short string
	var err error

	for {
		short, err = generateCode(8)
		if err != nil {
			return "", fmt.Errorf("ошибка при генерации короткой ссылки: {%w}", err)
		}

		if _, ok := lByS[short]; ok {
			continue
		}
		sByL[longURL] = short

		// Проверка, что в мапе уже есть значение
		// удаление, если есть
		for k, v := range lByS {
			if v == longURL {
				delete(lByS, k)
			}
		}

		lByS[short] = longURL
		break
	}
	return short, nil
}

// Функция генерирует строку случайных символов. Возвращает сгенерированную строку и ошибку.
//
// Параметры:
//
//	n - количество символов, из которых будет состоять строка.
func generateCode(n int) (string, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:n], nil
}

// Функция обновляет содержимое файла хранения данных. Возвращает ошибку.
//
// Параметры:
//
//	filename - полное имя файла.
//	mapShortByLong - мапа, данные из которой, будут переданы в файл.
//	logger - логгер.
func storageFileURL(filename string, mapShortByLong map[string]string, logger *zap.Logger) error {

	if logger == nil {
		return errors.New("в аргументе logger, функции storageFileURL, нет указателя")
	}
	if filename == "" {
		return errors.New("принят пустой путь к файлу хранения")
	}
	if mapShortByLong == nil {
		return errors.New("нет указателя на мапу")
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("ошибка <%v> открытия файла <%s>", err, filename)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Error("Ошибка при закрытии подключения к файлу",
				zap.Error(err))
		}
	}()

	// Вывод с построением строк
	file.WriteString("[\n")

	numb := 1
	size := len(mapShortByLong)
	for k, v := range mapShortByLong {

		baseURL := strings.Trim(k, "\"")

		str := fmt.Sprintf(`	{"uuid":"%s","short_url":"%s","original_url":"%s"}`, fmt.Sprintf("%d", numb), v, baseURL)
		file.WriteString(str)
		if numb < size {
			file.WriteString(",\n")
		}
		numb++
	}
	file.WriteString("\n")
	file.WriteString("]\n")

	return nil
}

// Функция выполняет сохранение в БД новой пары соответствия URL. Применяется ON CONFLICT. Возвращает ошибку.
//
// Параметры:
//
//	db - указатель на БД.
//	longURL - длинное представление исходного URL.
//	shortURL - значение сокращения URL.
func storageDBURLOnConflict(db *sql.DB, longURL, shortURL string) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("нет указателя на БД в аргументе db")
	}
	if longURL == "" {
		return errors.New("принято пустое значение longURL аргумента")
	}
	if shortURL == "" {
		return errors.New("принято пустое значение shortURL аргумента")
	}

	// Сохранение (обновление) пары соответствия в БД
	q := `
		INSERT INTO shortener (long, short) 
		VALUES ($1, $2) 
		ON CONFLICT (long) DO UPDATE 
		SET short = EXCLUDED.short
		RETURNING id;
		`

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := db.ExecContext(ctx, q, longURL, shortURL)
	if err != nil {
		return err
	}
	return nil
}

// Функция выполняет сохранение в БД новой пары соответствия URL, без ON CONFLICT. Возвращает ошибку.
//
// Параметры:
//
//	db - указатель на БД.
//	longURL - длинное представление исходного URL.
//	shortURL - значение сокращения URL.
//	uuid - ID.
func storageDBURLSimple(db *sql.DB, longURL, shortURL, uuid string) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("нет указателя на БД в аргументе db")
	}
	if longURL == "" {
		return errors.New("принято пустое значение longURL аргумента")
	}
	if shortURL == "" {
		return errors.New("принято пустое значение shortURL аргумента")
	}

	// Сохранение (обновление) пары соответствия в БД.
	q := `
		INSERT INTO shortener (long, short, uuid) 
		VALUES ($1, $2, $3) 
		`

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := db.ExecContext(ctx, q, longURL, shortURL, uuid)
	if err != nil {
		return err
	}

	return nil
}

// Функция выполняет сохранение в БД новой пары соответствия URL с использованием транзакции. Возвращает ошибку.
//
// Параметры:
//
//	tx - указатель на транзакцию.
//	longURL - длинное представление исходного URL.
//	shortURL - значение сокращения URL.
//	uuidRx - ID.
func storageDBURLtx(tx *sql.Tx, longURL, shortURL, uuidRx string) error {

	// Проверка аргументов
	if tx == nil {
		return errors.New("нет указателя на транзакцию в аргументе tx")
	}
	if longURL == "" {
		return errors.New("принято пустое значение longURL аргумента")
	}
	if shortURL == "" {
		return errors.New("принято пустое значение shortURL аргумента")
	}

	// Сохранение (обновление) пары соответствия в БД.
	str := `
		INSERT INTO shortener (long, short, uuid) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (long) DO UPDATE 
		SET short = EXCLUDED.short
		RETURNING id;
		`

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := tx.ExecContext(ctx, str, longURL, shortURL, uuidRx)
	if err != nil {
		return err
	}
	return nil
}

// Функция с последовательностью действий по подготовке и сохранению данных в БД.
// В запросе к БД нет обработки конфликта long. Возвращает короткое представление URL и ошибку.
//
// Парметры:
//
//	db - указатель на БД.
//	longURL - длинное представление URL.
//	uuid - ID.
func actionStorageDBURLSimple(db *sql.DB, longURL, uuid string) (string, error) {

	// Проверки акргументов
	if db == nil {
		return "", errors.New("нет указателя на БД")
	}
	if longURL == "" {
		return "", errors.New("пустое значение в longURL")
	}

	// Генерация кода сокращения.
	shortURL, err := generateCode(8)
	if err != nil {
		return "", fmt.Errorf("ошибка при генерации нового кода: <%w>", err)
	}

	// Сохранение в БД.
	err = storageDBURLSimple(db, longURL, shortURL, uuid)
	if err != nil {
		return "", err // ожидается появление ошибки по уникальности короткого представления.
	}

	return shortURL, nil
}

// Функция с последовательностью действий по подготовке и сохранению данных в БД с использованием транзакции. Возвращает короткое представление URL и ошибку.
//
// Парметры:
//
//	tx - указатель на транзакцию.
//	longURL - длинное представление URL.
//	uuidRx - ID.
func actionStorageDBURLtx(tx *sql.Tx, longURL, uuidRx string) (string, error) {

	// Проверки акргументов
	if tx == nil {
		return "", errors.New("нет указателя на tx")
	}
	if longURL == "" {
		return "", errors.New("пустое значение в longURL")
	}

	// Генерация кода сокращения.
	shortURL, err := generateCode(8)
	if err != nil {
		return "", fmt.Errorf("ошибка при генерации нового кода: <%w>", err)
	}

	// Сохранение в БД.
	err = storageDBURLtx(tx, longURL, shortURL, uuidRx)
	if err != nil {
		return "", err // ожидается появление ошибки по уникальности короткого представления.
	}

	return shortURL, nil
}

// Функция выполняет сохранение принятых данных в БД. Возвращает массив коротких ссылок и ошибку.
//
// Параметры:
//
//	db - указатель на БД.
//	batchLongURL - массив длинных ссылок.
//	baseAddrShortURL - базовый адрес.
//	uuidRx - ID.
func allActionsStorageBatchDBURL(db *sql.DB, batchLongURL []rxLongURLBatch, baseAddrShortURL, uuidRx string) ([]txShortURLBatch, error) {

	// Проверка аргументов.
	if db == nil {
		return nil, errors.New("нет указателя на БД")
	}
	if batchLongURL == nil {
		return nil, errors.New("нет указателя на batch")
	}
	if len(batchLongURL) == 0 {
		return nil, errors.New("в принятом массиве длинных ссылок нет данных")
	}

	errUniqueShort := `pq: duplicate key value violates unique constraint "shortener_short_key"` // ошибка по уникальности значений короткого представления.

	txData := make([]txShortURLBatch, 0)

	// Начало транзакции.
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("ошибка начала транзакции: <%w>", err)
	}
	defer func() {
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				log.Fatalf("аварийное прерывание работы приложения: ошибка при откате изменений в БД (метрики) <%v>", err)
			}
		}
	}()

	for _, v := range batchLongURL {
		ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
		defer cancel()

		done := false
		var shortURL string
		var err error

		for !done {
			select {
			case <-ctx.Done():
				return nil, errors.New("сработал контекст. превышено время выполнения")
			default:

				v.OriginalURL = strings.Trim(v.OriginalURL, "\"")

				shortURL, err = actionStorageDBURLtx(tx, v.OriginalURL, uuidRx)
				if err != nil && err.Error() == errUniqueShort { // проверка ошибки по уникальности короткого представления.
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("ошибка работы с БД: <%w>", err)
				}

				done = true
			}
		}

		// заполнение возвращаемого массива.
		var el txShortURLBatch
		el.CorrelationID = v.CorrelationID
		el.ShortURL = baseAddrShortURL + shortURL

		txData = append(txData, el)
	}

	// Подтверждение транзакции.
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("ошибка подтверждения транзакции: <%w>", err)
	}

	return txData, nil
}

// Функция выполняет работу с мапами. Возвращает ошибку.
//
// Параметры:
//
//	batchLongURL - массив длинных ссылок.
//	sByL - мапа где ключ - длинный URL, значение - короткое представление.
//	lByS - мапа где ключ - короткое представление, значение - длинный URL.
func storageBatchMap(batchLongURL []rxLongURLBatch, sByL, lByS map[string]string) error {

	// Проверка аргументов
	if batchLongURL == nil {
		return errors.New("нет указателя на batch")
	}
	if len(batchLongURL) == 0 {
		return errors.New("принят batch с пустым содержимым")
	}
	if sByL == nil {
		return errors.New("нет указателя на sByL")
	}
	if lByS == nil {
		return errors.New("нет указателя на lByS")
	}

	for _, v := range batchLongURL {

		_, err := fillListShortByLong(sByL, lByS, v.OriginalURL)
		if err != nil {
			return fmt.Errorf("функция fillListShortByLong вернула ошибку: <%w>", err)
		}
	}

	return nil
}

// Функция выполняет подготовку данных для возврата обработчиком запроса. Возвращает массив и ошибку.
//
// Парметры:
//
//	lByS - мапа где ключ - короткое представление, значение - длинный URL.
//	batchLongURL - принятый массив длинных ссылок.
//	conf - конфигурация.
func prapareBatchResponse(lByS map[string]string, batchLongURL []rxLongURLBatch, conf *ShortLong) ([]txShortURLBatch, error) {

	// Проверка аргументов.
	if conf == nil {
		return nil, errors.New("нет указателя на мапу conf")
	}
	if lByS == nil {
		return nil, errors.New("нет указателя на мапу lByS")
	}
	if batchLongURL == nil {
		return nil, errors.New("нет указателя на массив batchLongURL")
	}
	if len(batchLongURL) == 0 {
		return nil, errors.New("принят пустой массив batchLongURL")
	}

	// Наполнение массива.
	txData := make([]txShortURLBatch, 0)

	for _, v := range batchLongURL {

		var el txShortURLBatch

		v.OriginalURL = strings.Trim(v.OriginalURL, "\"")

		// Поиск короткой ссылки по длинной.
		for s, l := range lByS {

			if l == v.OriginalURL {
				el.CorrelationID = strings.Trim(v.CorrelationID, "\"")
				el.ShortURL = conf.BaseAddrShortURL + s

				txData = append(txData, el)
				break
			}
		}
	}

	return txData, nil
}

// Функция выполняет обработку принятого, исходного URL с сохранением в БД или в мапы и файл. В зависимости от настроек.
// Возвращается короткое представление, uuid и ошибку.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - конфигурация для работы сервиса сокращения.
//	rxLongURL - исходный URL.
//	uuidRx - ID.
func workWithRxData(db *sql.DB, sl *ShortLong, rxLongURL, uuidRx string) (short string, err error) {

	// Проверка аргументов.
	if sl == nil {
		return "", fmt.Errorf("в принятом аргументе sl, нет указателя")
	}
	if rxLongURL == "" {
		return "", fmt.Errorf("в принятом аргументе rxLongURL, нет содержимого")
	}
	if sl.List == nil {
		return "", fmt.Errorf("в принятом аргументе sl, нет указателя на мапы")
	}

	sl.DB.mu.RLock()
	defer sl.DB.mu.RUnlock()

	// Работа
	var shortURL string

	if db != nil { // сохранение пары соответствия в БД.

		shortURL, err = actionStorageDBURLSimple(db, rxLongURL, uuidRx)
		if err != nil {

			return "", fmt.Errorf("ошибка записи в БД: <%w>", err)
		}
	}

	if db == nil { // сохранение пары соответствия в мапы и файл.

		shortURL, err = fillListShortByLong(sl.List.ShorByLong, sl.List.LongByShort, rxLongURL)
		if err != nil {
			return "", fmt.Errorf("ошибка заполнения мап: <%w>", err)
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong, sl.Log)
		if err != nil {
			return "", fmt.Errorf("ошибка при сохранении в файл: <%w>", err)
		}
	}

	// Возврат результата.
	short = shortURL

	return short, nil
}

// Функция содержит логику обработчика ShortURLFromLong.
//
// Параметры:
//
//	db - указатель на БД
//	sl - конфигурация.
//	w - http.ResponseWriter.
//	r - *http.Request.
func internalShortURLFromLong(db *sql.DB, sl *ShortLong, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("В аргументе sl, функции internalShortURLFromLong, нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		sl.Log.Error("Нет указателя в аргументе r")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		sl.Log.Error("Нет указателя в аргументе w",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Получение тела запроса.
	rxLongURL, uuidRx, err := InternalShortURLFromLongLayerRx(r, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Обработка.
	result, flagConflict, err := InternalShortURLFromLongLayerWork(db, sl, rxLongURL, uuidRx)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Ответ.
	err = InternalShortURLFromLongLayerTx(w, result, flagConflict, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

// Функция содержит логику обработчика LongURLFromShort.
//
// Параметры:
//
//	db - указатель на БД
//	sl - конфигурация.
//	w - http.ResponseWriter.
//	r - *http.Request.
func internalLongURLFromShort(db *sql.DB, sl *ShortLong, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("В аргументе sl, функции internalLongURLFromShort, нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		sl.Log.Error("Ошибка в internalLongURLFromShort",
			zap.String("reason", "нет указателя на аргумент w"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		sl.Log.Error("Ошибка в internalShortURLFromLong",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Приём.
	short, err := internalLongURLFromShortLayerRx(r, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Логика.
	long, err := internalLongURLFromShortLayerWork(db, sl, short)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		case "404":
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
			return
		case "410":
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Ответ.
	internalLongURLFromShortLayerTx(w, long)
}

// Функция содержит логику обработчика ShortURLFromLongJSON.
//
// Параметры:
//
//	db - указатель на БД
//	sl - конфигурация.
//	w - http.ResponseWriter.
//	r - *http.Request.
func internalShortURLFromLongJSON(db *sql.DB, sl *ShortLong, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("В аргументе sl, функции internalShortURLFromLongJSON, нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		sl.Log.Error("Ошибка в internalShortURLFromLongJSON",
			zap.String("reason", "нет указателя на аргумент w"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		sl.Log.Error("Ошибка в internalShortURLFromLongJSON",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Content-Type") != `application/json` {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Приём.
	rxJSON, uuidRx, err := internalShortURLFromLongJSONLayerRx(r, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Логика.
	shortStr, flagConflict, err := internalShortURLFromLongJSONLayerWork(db, sl, rxJSON, uuidRx)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Ответ.
	err = internalShortURLFromLongJSONLayerTx(w, shortStr, flagConflict, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

// Функция с содержимым запроса к БД для получения короткого представления по исходному URL. Возвращается короткое представление и ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	longURL - длинное представление URL.
func readShortByLongDB(db *sql.DB, longURL string) (string, error) {

	// Проверка аргументов
	if db == nil {
		return "", errors.New("нет указателя в аргументе db")
	}
	if longURL == "" {
		return "", errors.New("в аргументе longURL нет содержимого")
	}

	// Логика
	var shortURL string

	query := `SELECT short FROM shortener WHERE long = $1`
	err := db.QueryRow(query, longURL).Scan(&shortURL)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("URL не найден: %s", longURL)
		}
		return "", fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}

	return shortURL, nil
}

// Функция выполняет запрос к БД. Возвращает длинное представление URL и ошибку.
//
// Параметры:
//
//	db - указатель на БД.
//	shortURL - сокращенное представление URL.
func readLongAndFlagByShortDB(db *sql.DB, shortURL string) (string, error) {

	// Проверка аргументов.
	if db == nil {
		return "", errors.New("в аргументе db нет указателя")
	}
	if shortURL == "" {
		return "", errors.New("в аргументе shortURL нет содержимого")
	}

	// Логика.
	var longURL string
	var deleteFlag bool

	query := `SELECT long, deleteflag FROM shortener WHERE short = $1`

	err := db.QueryRow(query, shortURL).Scan(&longURL, &deleteFlag)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("строка с: <%s> не найдена", shortURL)
		}
		return "", fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}

	// Возврат.
	if deleteFlag {
		return "", nil
	}
	return longURL, nil
}

// Функция содержит логику обработчика ShortURLFromLongBatch.
//
// Параметры:
//
//	db - указатель на БД
//	sl - конфигурация.
//	w - http.ResponseWriter.
//	r - *http.Request.
func internalShortURLFromLongBatch(db *sql.DB, sl *ShortLong, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if sl == nil {
		log.Println("В аргументе sl, функции internalShortURLFromLongBatch, нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		sl.Log.Error("Нет указателя в аргументе r")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		sl.Log.Error("Нет указателя в аргументе w",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Приём.
	rxLongURLBatch, uuidRx, err := internalShortURLFromLongBatchLayerRx(r, sl.Log)
	if err != nil {
		switch err.Error() {
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Обработка.
	batchShortURL, err := internalShortURLFromLongBatchLayerWork(db, sl, rxLongURLBatch, uuidRx)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Ответ.
	err = internalShortURLFromLongBatchLayerTx(w, batchShortURL, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

// Функция содержит логику обработчика DeleteUserURLs.
//
// Параметры:
//
//	db - указатель на БД
//	sl - конфигурация.
//	w - http.ResponseWriter.
//	r - *http.Request.
func internalDeleteUserURLs(db *sql.DB, sl *ShortLong, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов.
	if r == nil {
		sl.Log.Error("В аргументе r нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if db == nil {
		sl.Log.Error("В аргументе db нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		sl.Log.Error("В аргументе w нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Приём.
	rxArray, uuid, err := internalDeleteUserURLsLayerRx(r, sl.Log)
	if err != nil {
		switch err.Error() {
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Логика.
	err = internalDeleteUserURLsLayerWork(db, sl, rxArray, uuid)
	if err != nil {
		switch err.Error() {
		case "400":
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Ответ.
	internalDeleteUserURLsLayerTx(w)
}

// Функция помечает записи в БД, которые в будущем будут удалены. Возвращает ошибку..
//
// Параметры:
//
//	db - указатель на БД.
//	sl - конфигурация.
//	shortURLs - массив кротких URL.
//	uuid - авторизация пользователя.
func markFlagDelDB(db *sql.DB, sl *ShortLong, shortURLs []string, uuidRx string) error {

	// Проверка аргументов.
	if db == nil {
		return errors.New("нет указателя в аргументе db")
	}
	if shortURLs == nil {
		return errors.New("нет указателя в аргументе shortURLs")
	}
	if len(shortURLs) == 0 {
		return errors.New("нет данных в аргументе shortURLs")
	}

	// Логика.
	cnt := 0
	query := `UPDATE shortener SET deleteFlag = true WHERE short = $1 AND uuid = $2`

	for _, shortURL := range shortURLs {

		result, err := db.Exec(query, shortURL, uuidRx)
		if err != nil {
			sl.Log.Error("Ошибка при изменении значения флага deleteFlag",
				zap.Error(err),
				zap.String("short", shortURL),
				zap.String("uuidRx", uuidRx),
			)
			continue
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			sl.Log.Error("Ошибка при получении количества затронутых строк",
				zap.Error(err),
				zap.String("short", shortURL),
				zap.String("uuidRx", uuidRx),
			)
			continue
		}
		if rowsAffected > 0 { // проверка что deleteFlag взведён.

			var data DeleteDB

			data.Short = shortURL
			data.UUID = uuidRx

			sl.DB.ChForDelete <- data // Передача в go данных строки для удаления.

			cnt++ // Счёт переданных в go записей на удаление.
		}
	}

	if cnt > 0 {
		sl.DB.ChDoDelete <- struct{}{} // передача в go разрешения на запуск очистки таблицы.
	}

	return nil
}

// Функция содержит логику обработчика UserURLs.
//
// Параметры:
//
//	db - указатель на БД
//	sl - конфигурация.
//	w - http.ResponseWriter.
//	r - *http.Request.
func internalUserURLs(db *sql.DB, sl *ShortLong, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("В аргументе sl нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		sl.Log.Error("В аргументе r нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		sl.Log.Error("В аргументе w нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Логика.
	shortLong, err := internalUserURLsLayerWork(db, sl)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Ответ.
	err = internalUserURLsLayerTx(w, shortLong, sl.Log)
	if err != nil {
		switch err.Error() {
		case "500":
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		default:
			sl.Log.Error("Код ошибки не опознан",
				zap.String("code", err.Error()),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

// Функция выполняет чтение содержимого таблицы сокращения ссылок. Возвращает мапу, где ключ - короткое представление, значение - URL, и ошибку.
//
// Параметры:
//
//	db - указатель на БД.
//	log - логгер.
func GetAllShortenerDB(db *sql.DB, log *zap.Logger) (map[string]string, error) {

	// Проверка аргументов
	if db == nil {
		return nil, errors.New("в аргументе db нет указателя")
	}
	if log == nil {
		return nil, errors.New("в аргументе log нет указателя")
	}

	// Запрос
	shortToLongMap := make(map[string]string)

	rows, err := db.Query("SELECT short, long FROM shortener")
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Error("Ошибка закрытия rows",
				zap.Error(err),
			)
		}
	}()

	// Обрабатываем результаты.
	for rows.Next() {

		var short, long string

		if err := rows.Scan(&short, &long); err != nil {
			log.Error("ошибка сканирования строки",
				zap.Error(err),
			)
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		shortToLongMap[short] = long
	}

	if err := rows.Err(); err != nil {
		log.Error("ошибка при итерации по строкам",
			zap.Error(err),
		)
		return nil, fmt.Errorf("ошибка при итерации по строкам: %w", err)
	}

	return shortToLongMap, nil
}

// Функция выполняет очистку содержимого таблицы сокращения ссылок. Возвращает ошибку.
//
// Параметры:
//
//	db - указатель на БД.
func ClearShortenerTable(db *sql.DB) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("нет указателя в аргументе db")
	}

	query := `TRUNCATE TABLE shortener RESTART IDENTITY;`

	// Запрос.
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("ошибка при очистке таблицы: <%w>", err)
	}

	return nil
}

// Взаимодействие с наблюдателями.
//
// Параметры:
//
//	body - тело запроса.
//	path - путь запроса.
//	sl - параметры сервиса.
//	locationHeader - содержимое заголовка location
func processingObserver(body []byte, path string, sl *ShortLong, locationHeader string) {

	// Проверка аргументов
	if sl == nil {
		log.Println("В аргументе sl, функции processingObserver, нет указателя.")
		return
	}

	// Логика
	var dataBody string
	var auditEvent observer.AuditEvent

	switch path {
	case "/":

		dataBody = string(body)

		auditEvent = observer.AuditEvent{
			Timestamp: time.Now().Unix(),
			Action:    shorten,
			UserID:    "",
			URL:       dataBody,
		}
	case "/api/shorten":

		var dataBodyJSON rxLongURL

		if err := json.Unmarshal(body, &dataBodyJSON); err != nil {
			sl.Log.Error("Ошибка json.Unmarshal", zap.Error(err))
			return
		}
		dataBody = dataBodyJSON.URL

		auditEvent = observer.AuditEvent{
			Timestamp: time.Now().Unix(),
			Action:    shorten,
			UserID:    "",
			URL:       dataBody,
		}

	default: //`/{id}`.

		auditEvent = observer.AuditEvent{
			Timestamp: time.Now().Unix(),
			Action:    follow,
			UserID:    "",
			URL:       locationHeader,
		}
	}

	sl.Observer.Notify(auditEvent)
}
