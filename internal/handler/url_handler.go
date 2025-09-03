package handler

import (
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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/config/config"
	gz "github.com/Part001-R/YaPrShortener/internal/service/gzip"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type ShortLongURLT struct {
	ShorByLong  map[string]string
	LongByShort map[string]string
	Mu          sync.RWMutex
}

type ShortLongDBT struct {
	ptr *sql.DB
	Mu  sync.RWMutex
}

type ShortLongT struct {
	List             *ShortLongURLT
	DB               *ShortLongDBT
	BaseAddrShortURL string
	ServerAddr       string
	FileStoragePath  string
}

type EventURLT struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type rxLongURLT struct {
	URL string `json:"url"`
}

type txShortURLT struct {
	Result string `json:"result"`
}

type rxLongURLBatchT struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type txShortURLBatchT struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

type txShortURLOriginalURLT struct { // дополнительная реализация из файла теста 9
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type handlersI interface {
	ShortURLFromLong(w http.ResponseWriter, r *http.Request)
	LongURLFromShort(w http.ResponseWriter, r *http.Request)
	ShortURLFromLongJSON(w http.ResponseWriter, r *http.Request)
	ShortURLFromLongBatch(w http.ResponseWriter, r *http.Request)
	UserURLs(w http.ResponseWriter, r *http.Request)
	DeleteUserURLs(w http.ResponseWriter, r *http.Request)
}

type systemActI interface {
	PingDB(w http.ResponseWriter, r *http.Request)
}

type fileI interface {
	LoadFileURL() error
}

// основной интерфейс
type ShortLongI interface {
	systemActI
	handlersI
	fileI
}

func NewShortLongURL() *ShortLongURLT {
	return &ShortLongURLT{
		ShorByLong:  make(map[string]string),
		LongByShort: make(map[string]string),
		Mu:          sync.RWMutex{},
	}
}

func NewShortLongURLDB(db *sql.DB) *ShortLongDBT {
	return &ShortLongDBT{
		ptr: db,
		Mu:  sync.RWMutex{},
	}
}

func NewShortLongStorage(storage *ShortLongURLT, db *ShortLongDBT, Fl config.ConfigT) ShortLongI {
	return &ShortLongT{
		List:             storage,
		DB:               db,
		BaseAddrShortURL: Fl.BaseAddrShortURL,
		ServerAddr:       Fl.ServerAddr,
		FileStoragePath:  Fl.FileStoragePath,
	}
}

func (sl *ShortLongT) ShortURLFromLong(w http.ResponseWriter, r *http.Request) {

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Вся логика обработчика
	internalShortURLFromLong(sl.DB.ptr, sl, w, r)
}

func (sl *ShortLongT) ShortURLFromLongBatch(w http.ResponseWriter, r *http.Request) {

	sl.DB.Mu.RLock()
	defer sl.DB.Mu.RUnlock()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Вся логика обработчика
	internalShortURLFromLongBatch(sl.DB.ptr, sl, w, r)
}

func (sl *ShortLongT) LongURLFromShort(w http.ResponseWriter, r *http.Request) {

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

	internalLongURLFromShort(sl.DB.ptr, sl, w, r)
}

func (sl *ShortLongT) ShortURLFromLongJSON(w http.ResponseWriter, r *http.Request) {

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Вся логика обработчика
	internalShortURLFromLongJSON(sl.DB.ptr, sl, w, r)
}

func (sl *ShortLongT) LoadFileURL() error {

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

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
			logger.Log.Error("Ошибка при закрытии подключения к файлу",
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
	var events []EventURLT
	if err := json.Unmarshal(data, &events); err != nil {
		return fmt.Errorf("ошибка Unmarshal: %v", err)
	}

	for _, ev := range events {
		sl.List.ShorByLong[ev.OriginalURL] = ev.ShortURL
		sl.List.LongByShort[ev.ShortURL] = ev.OriginalURL
	}

	return nil
}

func (sl *ShortLongT) PingDB(w http.ResponseWriter, r *http.Request) {

	// Пинг
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := sl.DB.ptr.PingContext(ctx); err != nil {
		logger.Log.Error("Ошибка выполнения ping БД",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func Middleware(h http.HandlerFunc) http.HandlerFunc {
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
							logger.Log.Error("Ошибка при закрытии cw", zap.Error(err))
						}
					}()
					found = true
				case "identity":
					found = true
				default:
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
						logger.Log.Error("Ошибка в NewCompressReader",
							zap.Error(err),
							zap.String("method", r.Method),
							zap.String("url", r.URL.String()),
						)
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
					defer func() {
						if err := cr.Close(); err != nil {
							logger.Log.Error("Ошибка при закрытии cr", zap.Error(err))
						}
					}()
					defer func() {
						if err := r.Body.Close(); err != nil {
							logger.Log.Error("Ошибка при закрытии r.Body", zap.Error(err))
						}
					}()

					r.Body = cr
					found = true
				case "identity":
					found = true

				default:
				}
			}

			if !found {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}
		/*
			// Проверка куки
			cookie, err := r.Cookie("user_id")
			if err != nil || cookie.Value == "" {

				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			userID := cookie.Value
		*/

		// Запуск обработчика
		timeStart := time.Now()
		h(ow, r)
		duration := time.Since(timeStart)

		// Вывод в лог сводной информации по запросу
		logger.Log.Info("принят HTTP запрос",
			zap.String("URI", r.RequestURI),
			zap.String("метод", r.Method),
			zap.Duration("время выполнения запроса", duration),
		)
	})
}

func (sl *ShortLongT) UserURLs(w http.ResponseWriter, r *http.Request) {

	sl.DB.Mu.RLock()
	defer sl.DB.Mu.RUnlock()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	internalUserURLs(sl.DB.ptr, sl, w, r)

}

func (sl *ShortLongT) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {

	sl.DB.Mu.RLock()
	defer sl.DB.Mu.RUnlock()

	internalDeleteUserURLs(sl.DB.ptr, w, r)
}

// Функция наполняет мапы новыми парами соответствий длинных и коротких ссылок. Возвращает короткую ссылку и ошибку.
//
// Параметры:
//
// sByL - мапа, где в качестве ключа - длинный URL, значения - короткое представление.
// lByS - мапа, где в качестве ключа - короткое представление, значения - длинный URL.
// longURL - исходное, длинное значение URL.
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

		// Проверка, что в мапе есть уже значение
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
// n - количество символов, из которых будет состоять строка.
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
// filename - полное имя файла.
// mapShortByLong - мапа, данные из которой, будут переданы в файл.
func storageFileURL(filename string, mapShortByLong map[string]string) error {

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
			logger.Log.Error("Ошибка при закрытии подключения к файлу",
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

	/*
		// Реализация с выводом в одну строку

		var events []EventURLT
		numb := 1
		for k, v := range mapShortByLong {

			var event EventURLT
			event.UUID = numb
			event.OriginalURL = k
			event.ShortURL = v

			events = append(events, event)
			numb++
		}

		encoder := json.NewEncoder(file)

		if err := encoder.Encode(events); err != nil {
			return err
		}
	*/

	return nil
}

// Функция выполняет сохранение в БД новой пары соответствия URL. Применяется ON CONFLICT. Возвращает ошибку.
//
// Параметры:
//
// db - указатель на БД.
// longURL - длинное представление исходного URL.
// shortURL - значение сокращения URL.
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
// db - указатель на БД.
// longURL - длинное представление исходного URL.
// shortURL - значение сокращения URL.
func storageDBURLSimple(db *sql.DB, longURL, shortURL string) error {

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
		`

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := db.ExecContext(ctx, q, longURL, shortURL)
	if err != nil {
		return err
	}

	return nil
}

// Функция выполняет сохранение в БД новой пары соответствия URL с использованием транзакции. Возвращает ошибку.
//
// Параметры:
//
// tx - указатель на транзакцию.
// longURL - длинное представление исходного URL.
// shortURL - значение сокращения URL.
func storageDBURLtx(tx *sql.Tx, longURL, shortURL string) error {

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

	// Сохранение (обновление) пары соответствия в БД
	str := `
		INSERT INTO shortener (long, short) 
		VALUES ($1, $2) 
		ON CONFLICT (long) DO UPDATE 
		SET short = EXCLUDED.short
		RETURNING id;
		`

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := tx.ExecContext(ctx, str, longURL, shortURL)
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
// db - указатель на БД.
// longURL - длинное представление URL.
func actionStorageDBURLSimple(db *sql.DB, longURL string) (string, error) {

	// Проверки акргументов
	if db == nil {
		return "", errors.New("нет указателя на БД")
	}
	if longURL == "" {
		return "", errors.New("пустое значение в longURL")
	}

	// Генерация кода сокращения
	shortURL, err := generateCode(8)
	if err != nil {
		return "", fmt.Errorf("ошибка при генерации нового кода: <%w>", err)
	}

	// Сохранение в БД
	err = storageDBURLSimple(db, longURL, shortURL)
	if err != nil {
		return "", err // ожидается появление ошибки по уникальности короткого представления
	}

	return shortURL, nil
}

// Функция с последовательностью действий по подготовке и сохранению данных в БД с использованием транзакции. Возвращает короткое представление URL и ошибку.
//
// Парметры:
//
// tx - указатель на транзакцию.
// longURL - длинное представление URL.
func actionStorageDBURLtx(tx *sql.Tx, longURL string) (string, error) {

	// Проверки акргументов
	if tx == nil {
		return "", errors.New("нет указателя на tx")
	}
	if longURL == "" {
		return "", errors.New("пустое значение в longURL")
	}

	// Генерация кода сокращения
	shortURL, err := generateCode(8)
	if err != nil {
		return "", fmt.Errorf("ошибка при генерации нового кода: <%w>", err)
	}

	// Сохранение в БД
	err = storageDBURLtx(tx, longURL, shortURL)
	if err != nil {
		return "", err // ожидается появление ошибки по уникальности короткого представления
	}

	return shortURL, nil
}

// Функция выполняет сохранение принятых данных в БД. Возвращает массив коротких ссылок и ошибку.
//
// Параметры:
//
// db - указатель на БД.
// batchLongURL - массив длинных ссылок.
func allActionsStorageBatchDBURL(db *sql.DB, batchLongURL []rxLongURLBatchT, baseAddrShortURL string) ([]txShortURLBatchT, error) {

	// Проверка аргументов
	if db == nil {
		return nil, errors.New("нет указателя на БД")
	}
	if batchLongURL == nil {
		return nil, errors.New("нет указателя на batch")
	}
	if len(batchLongURL) == 0 {
		return nil, errors.New("в принятом массиве длинных ссылок нет данных")
	}

	errUniqueShort := `pq: duplicate key value violates unique constraint "shortener_short_key"` // ошибка по уникальности значений короткого представления

	txData := make([]txShortURLBatchT, 0)

	// Начало транзакции
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

				shortURL, err = actionStorageDBURLtx(tx, v.OriginalURL)
				if err != nil && err.Error() == errUniqueShort { // проверка ошибки по уникальности короткого представления
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("ошибка работы с БД: <%w>", err)
				}

				done = true
			}
		}

		// заполнение возвращаемого массива
		var el txShortURLBatchT
		el.CorrelationID = v.CorrelationID
		el.ShortURL = baseAddrShortURL + shortURL

		txData = append(txData, el)
	}

	// Подтверждение транзакции
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("ошибка подтверждения транзакции: <%w>", err)
	}

	return txData, nil
}

// Функция выполняет работу с мапами. Возвращает ошибку.
//
// Параметры:
//
// batchLongURL - массив длинных ссылок.
// sByL - мапа где ключ - длинный URL, значение - короткое представление.
// lByS - мапа где ключ - короткое представление, значение - длинный URL.
func storageBatchMap(batchLongURL []rxLongURLBatchT, sByL, lByS map[string]string) error {

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
// lByS - мапа где ключ - короткое представление, значение - длинный URL.
// batchLongURL - принятый массив длинных ссылок.
func prapareBatchResponse(lByS map[string]string, batchLongURL []rxLongURLBatchT, conf *ShortLongT) ([]txShortURLBatchT, error) {

	// Проверка аргументов
	if lByS == nil {
		return nil, errors.New("нет указателя на мапу lByS")
	}
	if batchLongURL == nil {
		return nil, errors.New("нет указателя на массив batchLongURL")
	}
	if len(batchLongURL) == 0 {
		return nil, errors.New("принят пустой массив batchLongURL")
	}

	// Наполнение массива
	txData := make([]txShortURLBatchT, 0)

	for _, v := range batchLongURL {

		var el txShortURLBatchT

		v.OriginalURL = strings.Trim(v.OriginalURL, "\"")

		// Поиск короткой ссылки по длинной
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
// Возвращается короткое представление и ошибка.
//
// Параметры:
//
// sl - конфигурация для работы сервиса сокращения.
// rxLongURL - исходный URL.
func workWithRxData(db *sql.DB, sl *ShortLongT, rxLongURL string) (string, error) {

	// Проверка аргументов
	if sl == nil {
		return "", fmt.Errorf("в принятом аргументе sl, нет указателя")
	}
	if rxLongURL == "" {
		return "", fmt.Errorf("в принятом аргументе rxLongURL, нет содержимого")
	}
	if sl.List == nil {
		return "", fmt.Errorf("в принятом аргументе sl, нет указателя на мапы")
	}
	if sl.DB == nil {
		return "", fmt.Errorf("в принятом аргументе sl, нет указателя на DB")
	}

	sl.DB.Mu.RLock()
	defer sl.DB.Mu.RUnlock()

	// Работа
	var shortURL string
	var err error

	if db != nil { // сохранение пары соответствия в БД

		shortURL, err = actionStorageDBURLSimple(db, rxLongURL)
		if err != nil {

			return "", fmt.Errorf("ошибка записи в БД: <%w>", err)
		}
	}

	if db == nil { // сохранение пары соответствия в мапы и файл

		shortURL, err = fillListShortByLong(sl.List.ShorByLong, sl.List.LongByShort, rxLongURL)
		if err != nil {
			return "", fmt.Errorf("ошибка заполнения мап: <%w>", err)
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong)
		if err != nil {
			return "", fmt.Errorf("ошибка при сохранении в файл: <%w>", err)
		}
	}

	return shortURL, nil
}

// Функция содержит логику обработчика ShortURLFromLong.
//
// Параметры:
//
// db - указатель на БД
// sl - конфигурация.
// w - http.ResponseWriter.
// r - *http.Request.
func internalShortURLFromLong(db *sql.DB, sl *ShortLongT, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLong",
			zap.String("reason", "нет указателя на аргумент w"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLong",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if sl == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLong",
			zap.String("reason", "нет указателя на аргумент sl"),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Логика
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Log.Error("Ошибка закрытия r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}
	}()
	if err != nil {
		logger.Log.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(rxData) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	rxLongURL := string(rxData)

	// Формирование короткого представления и сохранение
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // ошибка по уникальности значения длинного представления

	shortURL, err := workWithRxData(db, sl, rxLongURL)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(db, rxLongURL)
		if err != nil {
			logger.Log.Error("Ошибка при получении короткого представления по длинному URL",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("rxLongURL", rxLongURL),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Ответ
		strResult := sl.BaseAddrShortURL + shortURL

		w.Header().Set("Location", strResult)
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(strResult))
		return
	}
	if err != nil {
		logger.Log.Error("Ошибка в функции workWithRxData",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
			zap.String("rxLongURL", rxLongURL),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Ответ
	strResult := sl.BaseAddrShortURL + shortURL

	w.Header().Set("Location", strResult)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(strResult))
}

// Функция содержит логику обработчика LongURLFromShort.
//
// Параметры:
//
// db - указатель на БД
// sl - конфигурация.
// w - http.ResponseWriter.
// r - *http.Request.
func internalLongURLFromShort(db *sql.DB, sl *ShortLongT, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("Ошибка в internalLongURLFromShort",
			zap.String("reason", "нет указателя на аргумент w"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLong",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if sl == nil {
		logger.Log.Error("Ошибка в internalLongURLFromShort",
			zap.String("reason", "нет указателя на аргумент sl"),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Логика
	rxData := r.URL.Path[1:]
	if len(rxData) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	short := string(rxData)
	var long string
	var ok bool
	var err error

	if db != nil { // БД

		myErr := fmt.Sprintf("URL не найден: %s", short)

		long, err = readLongByShortDB(db, short)
		if err != nil && err.Error() == myErr {
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
			return
		}
		if err != nil {
			logger.Log.Error("Ошибка в функции readLongByShortDB",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("short", short),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	if db == nil { // Мапа

		long, ok = sl.List.LongByShort[short]
		if !ok {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		long = strings.Trim(long, "\"")
	}

	w.Header().Set("Location", long)
	w.WriteHeader(http.StatusTemporaryRedirect)

}

// Функция содержит логику обработчика ShortURLFromLongJSON.
//
// Параметры:
//
// db - указатель на БД
// sl - конфигурация.
// w - http.ResponseWriter.
// r - *http.Request.
func internalShortURLFromLongJSON(db *sql.DB, sl *ShortLongT, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongJSON",
			zap.String("reason", "нет указателя на аргумент w"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongJSON",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if sl == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongJSON",
			zap.String("reason", "нет указателя на аргумент sl"),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Content-Type") != `application/json` {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Логика
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Log.Error("Ошибка при закрытии r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}
	}()
	if err != nil {
		logger.Log.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(rxData) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var rxJSON = rxLongURLT{}
	err = json.Unmarshal(rxData, &rxJSON)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if rxJSON.URL == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// формирование короткого представления и сохранение
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // ошибка по уникальности значения длинного представления

	shortURL, err := workWithRxData(db, sl, rxJSON.URL)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(db, rxJSON.URL)
		if err != nil {
			logger.Log.Error("Ошибка в функции readShortByLongDB",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("longURL", rxJSON.URL),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Ответ

		strResult := sl.BaseAddrShortURL + shortURL
		var txJSON = txShortURLT{
			Result: strResult,
		}
		txData, err := json.Marshal(txJSON)
		if err != nil {
			logger.Log.Error("Ошибка сериализации данных",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write(txData)
		return

	}

	// Ответ
	strResult := sl.BaseAddrShortURL + shortURL
	var txJSON = txShortURLT{
		Result: strResult,
	}
	txData, err := json.Marshal(txJSON)
	if err != nil {
		logger.Log.Error("Ошибка сериализации данных",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(txData)
}

// Функция с содержимым запроса к БД для получения короткого представления по исходному URL. Возвращается короткое представление и ошибка.
//
// Параметры:
//
// db - указатель на БД.
// longURL - длинное представление URL.
func readShortByLongDB(db *sql.DB, longURL string) (string, error) {

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

// Функция с содержимым запроса к БД для получения короткого представления по исходному URL. Возвращается короткое представление и ошибка.
//
// Параметры:
//
// db - указатель на БД.
// longURL - длинное представление URL.
func readLongByShortDB(db *sql.DB, shortURL string) (string, error) {

	var longURL string

	query := `SELECT long FROM shortener WHERE short = $1`
	err := db.QueryRow(query, shortURL).Scan(&longURL)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("URL не найден: %s", shortURL)
		}
		return "", fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}

	return longURL, nil
}

// Функция содержит логику обработчика ShortURLFromLongBatch.
//
// Параметры:
//
// db - указатель на БД
// sl - конфигурация.
// w - http.ResponseWriter.
// r - *http.Request.
func internalShortURLFromLongBatch(db *sql.DB, sl *ShortLongT, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongBatch",
			zap.String("reason", "нет указателя на аргумент w"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongBatch",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if sl == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongBatch",
			zap.String("reason", "нет указателя на аргумент sl"),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Content-Type") != `application/json` {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение тела запроса
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Log.Error("Ошибка закрытия r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}

	}()
	if err != nil {
		logger.Log.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Десериализация принятых данных
	rxLongURLBatch := make([]rxLongURLBatchT, 0)

	err = json.Unmarshal(rxData, &rxLongURLBatch)
	if err != nil {

		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if len(rxLongURLBatch) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Обработка
	batchShortURL := make([]txShortURLBatchT, 0)

	if db != nil { // сохранение пары соответствия в БД

		batchShortURL, err = allActionsStorageBatchDBURL(db, rxLongURLBatch, sl.BaseAddrShortURL)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в БД",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	if db == nil { // сохранение пары соответствия в мапы и файл

		err = storageBatchMap(rxLongURLBatch, sl.List.ShorByLong, sl.List.LongByShort)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в мапы",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в файл",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		batchShortURL, err = prapareBatchResponse(sl.List.LongByShort, rxLongURLBatch, sl)
		if err != nil {
			logger.Log.Error("Ошибка при подготовке ответного batch",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Сериализация и ответ
	txData, err := json.Marshal(batchShortURL)
	if err != nil {
		logger.Log.Error("Ошибка при сериализации ответного batch",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(txData)
}

// Функция содержит логику обработчика DeleteUserURLs.
//
// Параметры:
//
// db - указатель на БД
// w - http.ResponseWriter.
// r - *http.Request.
func internalDeleteUserURLs(db *sql.DB, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("В аргументе r нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if db == nil {
		logger.Log.Error("В аргументе db нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		logger.Log.Error("В аргументе w нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Чтение тела запроса
	rxByteBody, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.Error("Ошибка чтения тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Log.Error("Ошибка закрытия r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}
	}()
	rxArray := make([]string, 0)
	if err := json.Unmarshal(rxByteBody, &rxArray); err != nil {
		logger.Log.Error("Ошибка сериализации данных",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Удаление данных
	deleteAllByShortDB(db, rxArray)

	w.WriteHeader(http.StatusAccepted)
}

// Функция с содержимым запроса к БД для получения короткого представления по исходному URL. Возвращается короткое представление и ошибка.
//
// Параметры:
//
// db - указатель на БД.
// shortURLs - массив кротких URL.
func deleteAllByShortDB(db *sql.DB, shortURLs []string) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("нет указателя в аргументе db")
	}
	if shortURLs == nil {
		return errors.New("нет указателя в аргументе shortURLs")
	}
	if len(shortURLs) == 0 {
		return errors.New("нет данных в аргументе shortURLs")
	}

	// Начало транзакции
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}

	// Удаление
	query := `DELETE FROM shortener WHERE short = $1`
	for _, shortURL := range shortURLs {
		_, err := tx.Exec(query, shortURL)
		if err != nil {
			logger.Log.Error("Ошибка при удалении данных",
				zap.Error(err),
				zap.String("short", shortURL),
			)
			tx.Rollback()
			return fmt.Errorf("ошибка удаления: <%s>: <%w>", shortURL, err)
		}
	}

	// Завершение транзакции
	if err := tx.Commit(); err != nil {
		logger.Log.Error("ошибка фиксации транзакции по удалению данных",
			zap.Error(err),
		)
		return fmt.Errorf("ошибка фиксации транзакции по удалению данных: %w", err)
	}

	return nil
}

// Функция содержит логику обработчика UserURLs.
//
// Параметры:
//
// db - указатель на БД
// sl - конфигурация.
// w - http.ResponseWriter.
// r - *http.Request.
func internalUserURLs(db *sql.DB, sl *ShortLongT, w http.ResponseWriter, r *http.Request) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("В аргументе r нет указателя")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if sl == nil {
		logger.Log.Error("В аргументе sl нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		logger.Log.Error("В аргументе w нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	txData := make([]byte, 0)
	_ = txData

	if db != nil { //БД

		shortLong, err := GetAllShortenerDB(db)
		if err != nil {
			logger.Log.Error("Ошибка в функции GetAllShortenerDB",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if len(shortLong) != 0 {
			txData, err = json.Marshal(shortLong)
			if err != nil {
				logger.Log.Error("Ошибка сериализации БД",
					zap.Error(err),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		} else {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusOK)
	}

	if db == nil { // Мапы

		el := txShortURLOriginalURLT{}
		shortLong := make([]txShortURLOriginalURLT, 0)

		for k, v := range sl.List.LongByShort {
			el.ShortURL = sl.BaseAddrShortURL + k
			el.OriginalURL = v

			shortLong = append(shortLong, el)
		}

		if len(shortLong) != 0 {
			sl.List.LongByShort = make(map[string]string) // очистка
		} else {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var err error

		txData, err = json.Marshal(shortLong)
		if err != nil {
			logger.Log.Error("Ошибка сериализации мапы",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	w.Write(txData)

}

func GetAllShortenerDB(db *sql.DB) (map[string]string, error) {

	shortToLongMap := make(map[string]string)

	// Запрос
	rows, err := db.Query("SELECT short, long FROM shortener")
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logger.Log.Error("Ошибка закрытия rows",
				zap.Error(err),
			)
		}
	}()

	// Обрабатываем результаты
	for rows.Next() {

		var short, long string

		if err := rows.Scan(&short, &long); err != nil {
			logger.Log.Error("ошибка сканирования строки",
				zap.Error(err),
			)
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		shortToLongMap[short] = long
	}

	if err := rows.Err(); err != nil {
		logger.Log.Error("ошибка при итерации по строкам",
			zap.Error(err),
		)
		return nil, fmt.Errorf("ошибка при итерации по строкам: %w", err)
	}

	return shortToLongMap, nil
}

// Вспомогательная функция для отладки работы приложения.
//
// Параметры:
//
// str - строка, для записи в файл.
/*
func WriteInFileDebugData(str string) {
	filename := "debug.txt"

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("ошибка <%v> открытия файла <%s>", err, filename)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.WriteString(str + "\n"); err != nil {
		log.Fatalf("ошибка <%v> записи в файл <%s>", err, filename)
	}
}
*/
