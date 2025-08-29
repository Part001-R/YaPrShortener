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
	DSN string
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

func NewShortLongURLDB(dsn string) *ShortLongDBT {
	return &ShortLongDBT{
		DSN: dsn,
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

	// Подключение к БД
	var db *sql.DB
	var err error

	if sl.DB.DSN != "" {

		db, err = sql.Open("postgres", sl.DB.DSN)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = db.Close()
		}()
	}

	// Вся логика обработчика
	internalShortURLFromLong(db, sl, w, r)
}

func (sl *ShortLongT) ShortURLFromLongBatch(w http.ResponseWriter, r *http.Request) {

	sl.DB.Mu.RLock()
	defer sl.DB.Mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain")

	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение тела запроса
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		_ = r.Body.Close()
	}()
	if err != nil {
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
	_ = batchShortURL // чтобы редактор не подчёркивал желтым, как неиспользуемую

	if sl.DB.DSN != "" { // сохранение пары соответствия в БД

		db, err := sql.Open("postgres", sl.DB.DSN)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = db.Close()
		}()

		batchShortURL, err = allActionsStorageBatchDBURL(db, rxLongURLBatch)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

	} else { // сохранение пары соответствия в мапы и файл

		err = storageBatchMap(rxLongURLBatch, sl.List.ShorByLong, sl.List.LongByShort)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		batchShortURL, err = prapareBatchResponse(sl.List.LongByShort, rxLongURLBatch)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Сериализация и ответ
	txData, err := json.Marshal(batchShortURL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(txData)
}

func (sl *ShortLongT) LongURLFromShort(w http.ResponseWriter, r *http.Request) {

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxData := r.URL.Path[1:]
	if len(rxData) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	short := string(rxData)

	long, ok := sl.List.LongByShort[short]
	if !ok {

		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	long = strings.Trim(long, "\"")

	w.Header().Set("Location", long)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (sl *ShortLongT) ShortURLFromLongJSON(w http.ResponseWriter, r *http.Request) {

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	// Подключение к БД
	var db *sql.DB
	var err error

	if sl.DB.DSN != "" {

		db, err = sql.Open("postgres", sl.DB.DSN)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = db.Close()
		}()
	}

	// Вся логика обработчика
	internalShortURLFromLongJSON(db, sl, w, r)
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
	defer file.Close()

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

	if sl.DB.DSN == "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Подключение
	db, err := sql.Open("postgres", sl.DB.DSN)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = db.Close()
	}()

	// Пинг
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
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

	sl.List.Mu.RLock()
	defer sl.List.Mu.RUnlock()

	sl.BaseAddrShortURL = strings.TrimSuffix(sl.BaseAddrShortURL, "/")
	sl.BaseAddrShortURL = sl.BaseAddrShortURL + "/"

	el := txShortURLOriginalURLT{}

	shortLong := make([]txShortURLOriginalURLT, 0)

	for k, v := range sl.List.LongByShort {
		el.ShortURL = sl.BaseAddrShortURL + k
		el.OriginalURL = v

		shortLong = append(shortLong, el)
	}

	w.Header().Set("Content-Type", "application/json")

	if len(shortLong) != 0 {
		w.WriteHeader(http.StatusOK)
		sl.List.LongByShort = make(map[string]string) // очистка
	} else {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	txData, err := json.Marshal(shortLong)
	if err != nil {

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(txData)
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
		_ = file.Close()
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
func allActionsStorageBatchDBURL(db *sql.DB, batchLongURL []rxLongURLBatchT) ([]txShortURLBatchT, error) {

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
		el.ShortURL = shortURL

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
func prapareBatchResponse(lByS map[string]string, batchLongURL []rxLongURLBatchT) ([]txShortURLBatchT, error) {

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
				el.ShortURL = s

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
	if db == nil && sl.DB.DSN != "" {
		return "", fmt.Errorf("в принятом аргументе db, нет указателя")
	}
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

	if sl.DB.DSN != "" { // сохранение пары соответствия в БД

		shortURL, err = actionStorageDBURLSimple(db, rxLongURL)
		if err != nil {
			return "", fmt.Errorf("ошибка записи в БД: <%w>", err)
		}

	} else { // сохранение пары соответствия в мапы и файл

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
	if sl == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if db == nil && sl.DB.DSN != "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if w == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Логика
	w.Header().Set("Content-Type", "text/plain")

	rxData, err := io.ReadAll(r.Body)
	defer func() {
		_ = r.Body.Close()
	}()
	if err != nil {
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Ответ
	strResult := sl.BaseAddrShortURL + shortURL

	w.Header().Set("Location", strResult)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(strResult))
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
	if db == nil && sl.DB.DSN != "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if sl == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if r.Header.Get("Content-Type") != `application/json` {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Логика
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		_ = r.Body.Close()
	}()
	if err != nil {
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

		shortURL, err := readShortByLongDB(db, rxJSON.URL)
		if err != nil {

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
