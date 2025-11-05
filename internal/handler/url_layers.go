// handler пакет. Секция слоёв обработчиков контроллера.
package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/Part001-R/YaPrShortener/internal/service/authoriz"
	"go.uber.org/zap"
)

// --------------------------------
// --- internalShortURLFromLong ---
// --------------------------------

// InternalShortURLFromLongLayerRx слой приёма данных для обработчика ShortURLFromLong. Возвращаетется длинное представление, ID и ошибка.
//
// Параметры:
//
//	r - интерфейс приёма.
//	logger - логгер.
func InternalShortURLFromLongLayerRx(r *http.Request, logger *zap.Logger) (longURL, uuid string, err error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("В аргументе log, функции InternalShortURLFromLongLayerRx, нет указателя.")
		return "", "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if r == nil {
		logger.Error("в аргементе r нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return "", "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Чтение тела запроса.
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Error("Ошибка закрытия r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}
	}()
	if err != nil {
		logger.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return "", "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		return "", "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Результат.
	uuid = r.Header.Get("Authorization")
	longURL = string(rxData)

	return longURL, uuid, nil
}

// InternalShortURLFromLongLayerWork слой основной логики для обработчика ShortURLFromLong. Возвращается результат работы, признак конфликта и ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - указатель на объект сервиса.
//	longURL - принятое длинное представление URL.
//	uuidRx - принятый ID.
func InternalShortURLFromLongLayerWork(db *sql.DB, sl *ShortLong, longURL, uuidRx string) (result string, flagConflict bool, err error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("В аргументе sl, функции InternalShortURLFromLongLayerWork, нет указателя")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if longURL == "" {
		sl.Log.Error("в аргементе longURL нет данных")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // Ошибка по уникальности значения длинного представления.

	shortURL, err := workWithRxData(db, sl, longURL, uuidRx)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(db, longURL)
		if err != nil {
			sl.Log.Error("Ошибка при получении короткого представления по длинному URL",
				zap.Error(err),
				zap.String("longURL", longURL),
			)
			err = fmt.Errorf("%d", http.StatusInternalServerError)
			flagConflict = false
			return "", flagConflict, err
		}

		// Ответ.
		// Конфиликт longURL.
		strResult := sl.BaseAddrShortURL + shortURL

		result = strResult
		flagConflict = true
		return result, flagConflict, nil
	}
	if err != nil {
		sl.Log.Error("Ошибка в функции workWithRxData",
			zap.Error(err),
		)
		err = fmt.Errorf("%d", http.StatusInternalServerError)
		flagConflict = false
		return "", flagConflict, err
	}

	// Ответ.
	// Конфликта нет.
	strResult := sl.BaseAddrShortURL + shortURL

	result = strResult
	flagConflict = false
	return result, flagConflict, nil
}

// InternalShortURLFromLongLayerTx слой формирования ответа для обработчика ShortURLFromLong. Возвращается ошибка.
//
// Параметры:
//
//	w - интерфейс ответа.
//	str - данные для отправки.
//	flagConflict - признак конфликта.
//	logger - логгер.
func InternalShortURLFromLongLayerTx(w http.ResponseWriter, str string, flagConflict bool, logger *zap.Logger) error {

	// Проверка аргументов.
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if str == "" {
		logger.Error("в аргементе str нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	if flagConflict { // Если запись существует.
		w.Header().Set("Location", str)
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(str))
		return nil
	}

	w.Header().Set("Location", str)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(str))

	return nil
}

// -------------------------------------
// --- internalShortURLFromLongBatch ---
// -------------------------------------

// internalShortURLFromLongBatchLayerRx слой обработки принятых данных запроса, для обработчика ShortURLFromLongBatch. Возвращается принятый набор, ID запроса и ошибка.
//
// Параметры:
//
//	r - интерфейс приёма данных.
//	logger - логгер.
func internalShortURLFromLongBatchLayerRx(r *http.Request, logger *zap.Logger) (rxLongBatch []rxLongURLBatch, uuidRx string, err error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("В аргументе logger, функции internalShortURLFromLongBatchLayerRx, нет указателя.")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if r == nil {
		logger.Error("Ошибка в internalShortURLFromLongBatchLayerRx",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Чтение тела запроса.
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Error("Ошибка закрытия r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}

	}()
	if err != nil {
		logger.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Десериализация принятых данных.
	rxLongURLBatch := make([]rxLongURLBatch, 0)

	err = json.Unmarshal(rxData, &rxLongURLBatch)
	if err != nil {
		logger.Error("Ошибка десериализации",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusBadRequest)
	}
	if len(rxLongURLBatch) == 0 {
		return nil, "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Возврат.
	rxLongBatch = rxLongURLBatch
	uuidRx = r.Header.Get("Authorization")
	return rxLongBatch, uuidRx, nil

}

// internalShortURLFromLongBatchLayerWork слой основной логики для обработчика ShortURLFromLongBatch. Возвращается набор сокращённых URL и ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - указатель на экземпляр сервиса.
//	longBatch - принятый набор длинных URL.
//	uuidRx - принятый ID.
func internalShortURLFromLongBatchLayerWork(db *sql.DB, sl *ShortLong, longBatch []rxLongURLBatch, uuidRx string) ([]txShortURLBatch, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргементе sl, функции internalShortURLFromLongBatchLayerWork, нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if longBatch == nil {
		sl.Log.Error("в аргементе longBatch нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(longBatch) == 0 {
		sl.Log.Error("в аргементе longBatch нет данных")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	batchShortURL := make([]txShortURLBatch, 0)
	var err error

	if db != nil { // БД.

		batchShortURL, err = allActionsStorageBatchDBURL(db, longBatch, sl.BaseAddrShortURL, uuidRx)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в БД",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}
	}

	if db == nil { // Мапы.

		err = storageBatchMap(longBatch, sl.List.ShorByLong, sl.List.LongByShort)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в мапы",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong, sl.Log)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в файл",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

		batchShortURL, err = prapareBatchResponse(sl.List.LongByShort, longBatch, sl)
		if err != nil {
			sl.Log.Error("Ошибка при подготовке ответного batch",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}
	}

	// Результат.
	return batchShortURL, nil
}

// internalShortURLFromLongBatchLayerTx слой реализации ответа, для обработчика ShortURLFromLongBatch. Возвращается ошибка.
//
// Парамметры:
//
//	w - интерфейс ответа.
//	shortBatch - массив сокращённого продставления.
//	logger - логгер.
func internalShortURLFromLongBatchLayerTx(w http.ResponseWriter, shortBatch []txShortURLBatch, logger *zap.Logger) error {

	// Проверка аргументов.
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if shortBatch == nil {
		logger.Error("в аргементе shortBatch нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(shortBatch) == 0 {
		logger.Error("в аргементе shortBatch нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Сериализация.
	txData, err := json.Marshal(shortBatch)
	if err != nil {
		logger.Error("Ошибка при сериализации ответного batch",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Ответ.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(txData)

	return nil
}

// ------------------------------------
// --- internalShortURLFromLongJSON ---
// ------------------------------------

// internalShortURLFromLongJSONLayerRx слой обработки данных запроса для обработчика ShortURLFromLongJSON. Возвращается длинное представление URL, ID и ошибка.
//
// Параметры:
//
//	r - интерфейс приёма.
//	logger - логгер.
func internalShortURLFromLongJSONLayerRx(r *http.Request, logger *zap.Logger) (rxLong rxLongURL, uuidRx string, err error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("в аргументе logger, функции internalShortURLFromLongJSONLayerRx, нет указателя")
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Error("Ошибка при закрытии r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}
	}()
	if err != nil {
		logger.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	var rxJSON = rxLongURL{}
	err = json.Unmarshal(rxData, &rxJSON)
	if err != nil {
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusBadRequest)
	}
	if rxJSON.URL == "" {
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Результат.
	uuidRx = r.Header.Get("Authorization")
	rxLong = rxJSON

	return rxLong, uuidRx, nil
}

// internalShortURLFromLongJSONLayerWork слой основной логики для обработчика ShortURLFromLongJSON. Возвращается короткое представление URL, флаг конфликта и ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - указатель на сервис.
//	rxJSON - принятое значение длинного URL.
//	uuidRx - принятый ID.
func internalShortURLFromLongJSONLayerWork(db *sql.DB, sl *ShortLong, rxJSON rxLongURL, uuidRx string) (short string, flagConflict bool, err error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргементе sl, функции internalShortURLFromLongJSONLayerWork, нет указателя")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if rxJSON.URL == "" {
		sl.Log.Error("в аргементе rxJSON.URL нет данных")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // Ошибка по уникальности значения длинного представления.

	shortURL, err := workWithRxData(db, sl, rxJSON.URL, uuidRx)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(db, rxJSON.URL)
		if err != nil {
			sl.Log.Error("Ошибка в функции readShortByLongDB",
				zap.Error(err),
				zap.String("longURL", rxJSON.URL),
			)
			return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
		}

		// Ответ.
		flagConflict = true
		short = sl.BaseAddrShortURL + shortURL
		return short, flagConflict, nil
	}

	// Ответ.
	flagConflict = false
	short = sl.BaseAddrShortURL + shortURL
	return short, flagConflict, nil
}

// internalShortURLFromLongJSONLayerTx слой иеализации ответа для обработчика ShortURLFromLongJSON. Возвращается ошибка.
//
// Параметры:
//
//	w - интерфейс ответа.
//	short - короткое представление.
//	flagConflict - флаг конфликта.
//	logger - логгер.
func internalShortURLFromLongJSONLayerTx(w http.ResponseWriter, short string, flagConflict bool, logger *zap.Logger) error {

	// Проверка аргументов.
	if logger == nil {
		log.Println("В аргументе logger, функции internalShortURLFromLongJSONLayerTx, нет указателя.")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if short == "" {
		logger.Error("в аргементе short нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	var txJSON = txShortURL{
		Result: short,
	}
	txData, err := json.Marshal(txJSON)
	if err != nil {
		logger.Error("Ошибка сериализации данных",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Ответ.
	if flagConflict {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write(txData)
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(txData)
	return nil
}

// ------------------------
// --- internalUserURLs ---
// ------------------------

// internalUserURLsLayerWork слой основной логики для обработчика UserURLs. Возвращается массив пар соответствий и ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - указателль на ссервис.
func internalUserURLsLayerWork(db *sql.DB, sl *ShortLong) ([]txShortURLOriginalURL, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Printf("в аргементе sl, функции internalUserURLsLayerWork, нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	el := txShortURLOriginalURL{}
	shortLong := make([]txShortURLOriginalURL, 0)

	if db != nil { // БД.

		shortLongDB, err := GetAllShortenerDB(db, sl.Log)
		if err != nil {
			sl.Log.Error("Ошибка в функции GetAllShortenerDB",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

		for k, v := range shortLongDB {
			el.ShortURL = sl.BaseAddrShortURL + k
			el.OriginalURL = v

			shortLong = append(shortLong, el)
		}

		if err := ClearShortenerTable(db); err != nil { // Очистка таблицы.
			sl.Log.Error("Ошибка в функции ClearShortenerTable",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

	}

	if db == nil { // Мапы.

		for k, v := range sl.List.LongByShort {
			el.ShortURL = sl.BaseAddrShortURL + k
			el.OriginalURL = v

			shortLong = append(shortLong, el)
		}

		sl.List.LongByShort = make(map[string]string) // Очистка мапы.
	}

	// Результат.
	return shortLong, nil
}

// internalUserURLsLayerTx слой передачи ответа для обработчика UserURLs. Возвращается ошибка.
//
// Параметры:
//
//	w - интерфейс ответа.
//	shortLong - массив пар соответствий.
//	logger - логгер.
func internalUserURLsLayerTx(w http.ResponseWriter, shortLong []txShortURLOriginalURL, logger *zap.Logger) error {

	// Проверка аргументов.
	if logger == nil {
		log.Println("в аргументе logger, функции internalUserURLsLayerTx, нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	w.Header().Set("Content-Type", "application/json")

	if len(shortLong) == 0 {

		// Ответ.
		uuid := authoriz.GenerateUniqueID()
		authoriz.SetUserCookie(w, uuid)

		authoriz.UUID = uuid

		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	txData, err := json.Marshal(shortLong)
	if err != nil {
		logger.Error("Ошибка сериализации ответа",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Ответ.
	w.WriteHeader(http.StatusOK)
	w.Write(txData)

	return nil
}

// ------------------------------
// --- internalDeleteUserURLs ---
// ------------------------------

// internalDeleteUserURLsLayerRx обработка принятых данных запроса, для обработчика DeleteUserURLs. Возвращается массив принятых данных, ID и ошибка.
//
// Параметры:
//
//	r - интерфейс приёма.
//	logger - логгер.
func internalDeleteUserURLsLayerRx(r *http.Request, logger *zap.Logger) (rxArr []string, uuidRx string, err error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("В аргументе logger, функции internalDeleteUserURLsLayerRx, нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	uuidRx = r.Header.Get("Authorization")

	// Применение json.Decoder для оптимизации.
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&rxArr); err != nil {
		logger.Error("Ошибка сериализации данных",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusBadRequest)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Error("Ошибка закрытия r.Body",
				zap.Error(err),
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
			)
		}
	}()

	// Ответ.
	return rxArr, uuidRx, nil
}

// internalDeleteUserURLsLayerWork основная логика для обработчика DeleteUserURLs. Возвращается ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - указатель на сервис.
//	rxData - массив принятых данных.
//	uuidRx - принятый ID.
func internalDeleteUserURLsLayerWork(db *sql.DB, sl *ShortLong, rxData []string, uuidRx string) error {

	// Проверка аргументов.
	if sl == nil {
		log.Println("В аргументе sl, функции internalDeleteUserURLsLayerWork, нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if rxData == nil {
		sl.Log.Error("в аргументе rxData нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		sl.Log.Error("в аргументе rxData нет данных")
		return fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Логика.
	if err := markFlagDelDB(db, sl, rxData, uuidRx); err != nil {
		sl.Log.Error("Ошибка при обновлении значения флагов daleteFlag",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	return nil
}

// internalDeleteUserURLsLayerTx слой реализации ответа для обработчика DeleteUserURLs.
//
// Параметры:
//
//	w - интерфейс ответа.
func internalDeleteUserURLsLayerTx(w http.ResponseWriter) {

	w.WriteHeader(http.StatusAccepted)
}

// --------------------------------
// --- internalLongURLFromShort ---
// --------------------------------

// internalLongURLFromShortLayerRx слой приёма данных запроса для обработчика LongURLFromShort. Возвращается принятое значение и ошибка.
//
// Параметры:
//
//	r - интерфейс приёма.
//	logger - логгер.
func internalLongURLFromShortLayerRx(r *http.Request, logger *zap.Logger) (string, error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("В аргументе logger, функции internalLongURLFromShortLayerRx, нет указателя.")
		return "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	rxData := r.URL.Path[1:]
	if len(rxData) == 0 {
		return "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Возврат.
	return rxData, nil
}

// internalLongURLFromShortLayerWork слой логики обработчика LongURLFromShort. Возвращется сформированное значение длинного URL и ошибка.
//
// Параметры:
//
//	db - указатель на БД.
//	sl - указатель сервиса.
//	short - принятое сокращённое значение.
func internalLongURLFromShortLayerWork(db *sql.DB, sl *ShortLong, short string) (string, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргументе sl, функции internalLongURLFromShortLayerWork, нет указателя")
		return "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if short == "" {
		sl.Log.Error("в аргументе rxData нет данных")
		return "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Логика.
	var long string
	var err error
	var ok bool

	if db != nil { // БД.

		myErr := fmt.Sprintf("строка с: <%s> не найдена", short)

		long, err = readLongAndFlagByShortDB(db, short)
		if err != nil && err.Error() == myErr {
			return "", fmt.Errorf("%d", http.StatusNotFound) // Если запись в БД нет.
		}
		if err != nil {
			sl.Log.Error("Ошибка в функции readLongAndFlagByShortDB",
				zap.Error(err),
			)
			return "", fmt.Errorf("%d", http.StatusInternalServerError)
		}

		if long == "" {
			return "", fmt.Errorf("%d", http.StatusGone) // Если запись есть, но взведён флаг deleteflag.
		}
	}

	if db == nil { // Мапа.

		long, ok = sl.List.LongByShort[short]
		if !ok {
			sl.Log.Error("в аргументе rxData нет данных")
			return "", fmt.Errorf("%d", http.StatusBadRequest)
		}
		long = strings.Trim(long, "\"")
	}

	// Возврат.
	return long, nil
}

// internalLongURLFromShortLayerTx слой формироания ответа обраблтчика LongURLFromShort.
//
// Параметры:
//
//	w - интерфейс ответа.
//	long - длинное представление URL.
func internalLongURLFromShortLayerTx(w http.ResponseWriter, long string) {

	w.Header().Set("Location", long)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
