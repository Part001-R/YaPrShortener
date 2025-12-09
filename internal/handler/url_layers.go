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
		return "", "", ErrStatusInternalServerError
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return "", "", ErrStatusInternalServerError
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
		return "", "", ErrStatusInternalServerError
	}
	if len(rxData) == 0 {
		return "", "", ErrStatusBadRequest
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
		return "", false, ErrStatusInternalServerError
	}
	if longURL == "" {
		sl.Log.Error("в аргементе longURL нет данных")
		return "", false, ErrStatusInternalServerError
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
			flagConflict = false
			return "", flagConflict, ErrStatusInternalServerError
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
		flagConflict = false
		return "", flagConflict, ErrStatusInternalServerError
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
		return ErrStatusInternalServerError
	}
	if str == "" {
		logger.Error("в аргементе str нет данных")
		return ErrStatusInternalServerError
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
		return nil, "", ErrStatusInternalServerError
	}
	if r == nil {
		logger.Error("Ошибка в internalShortURLFromLongBatchLayerRx",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		return nil, "", ErrStatusInternalServerError
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
		return nil, "", ErrStatusInternalServerError
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
		return nil, "", ErrStatusBadRequest
	}
	if len(rxLongURLBatch) == 0 {
		return nil, "", ErrStatusBadRequest
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
		return nil, ErrStatusInternalServerError
	}
	if longBatch == nil {
		sl.Log.Error("в аргементе longBatch нет указателя")
		return nil, ErrStatusInternalServerError
	}
	if len(longBatch) == 0 {
		sl.Log.Error("в аргементе longBatch нет данных")
		return nil, ErrStatusInternalServerError
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
			return nil, ErrStatusInternalServerError
		}
	}

	if db == nil { // Мапы.

		err = storageBatchMap(longBatch, sl.List.ShorByLong, sl.List.LongByShort)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в мапы",
				zap.Error(err),
			)
			return nil, ErrStatusInternalServerError
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong, sl.Log)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в файл",
				zap.Error(err),
			)
			return nil, ErrStatusInternalServerError
		}

		batchShortURL, err = prapareBatchResponse(sl.List.LongByShort, longBatch, sl)
		if err != nil {
			sl.Log.Error("Ошибка при подготовке ответного batch",
				zap.Error(err),
			)
			return nil, ErrStatusInternalServerError
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
		return ErrStatusInternalServerError
	}
	if shortBatch == nil {
		logger.Error("в аргементе shortBatch нет указателя")
		return ErrStatusInternalServerError
	}
	if len(shortBatch) == 0 {
		logger.Error("в аргементе shortBatch нет данных")
		return ErrStatusInternalServerError
	}

	// Сериализация.
	txData, err := json.Marshal(shortBatch)
	if err != nil {
		logger.Error("Ошибка при сериализации ответного batch",
			zap.Error(err),
		)
		return ErrStatusInternalServerError
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
func internalShortURLFromLongJSONLayerRx(r *http.Request, logger *zap.Logger) (rxLong RxLongURL, uuidRx string, err error) {

	// Проверка аргументов.
	if logger == nil {
		log.Println("в аргументе logger, функции internalShortURLFromLongJSONLayerRx, нет указателя")
		return RxLongURL{}, "", ErrStatusInternalServerError
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return RxLongURL{}, "", ErrStatusInternalServerError
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
		return RxLongURL{}, "", ErrStatusInternalServerError
	}
	if len(rxData) == 0 {
		return RxLongURL{}, "", ErrStatusBadRequest
	}

	var rxJSON = RxLongURL{}
	err = json.Unmarshal(rxData, &rxJSON)
	if err != nil {
		return RxLongURL{}, "", ErrStatusBadRequest
	}
	if rxJSON.URL == "" {
		return RxLongURL{}, "", ErrStatusBadRequest
	}

	// Результат.
	uuidRx = r.Header.Get("Authorization")
	rxLong = rxJSON

	return rxLong, uuidRx, nil
}

// InternalShortURLFromLongJSONLayerWork слой основной логики для обработчика ShortURLFromLongJSON. Возвращается короткое представление URL, флаг конфликта и ошибка.
//
// Параметры:
//
//	sl - указатель на сервис.
//	rxJSON - принятое значение длинного URL.
//	uuidRx - принятый ID.
func InternalShortURLFromLongJSONLayerWork(sl *ShortLong, rxJSON RxLongURL, uuidRx string) (short string, flagConflict bool, err error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргементе sl, функции internalShortURLFromLongJSONLayerWork, нет указателя")
		return "", false, ErrStatusInternalServerError
	}
	if rxJSON.URL == "" {
		sl.Log.Error("в аргементе rxJSON.URL нет данных")
		return "", false, ErrStatusInternalServerError
	}

	sl.muF.muInternalShortURLFromLongJSONLayerWork.Lock()
	defer sl.muF.muInternalShortURLFromLongJSONLayerWork.Unlock()

	// Логика.
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // Ошибка по уникальности значения длинного представления.

	shortURL, err := workWithRxData(sl.DB.Ptr, sl, rxJSON.URL, uuidRx)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(sl.DB.Ptr, rxJSON.URL)
		if err != nil {
			sl.Log.Error("Ошибка в функции readShortByLongDB",
				zap.Error(err),
				zap.String("longURL", rxJSON.URL),
			)
			return "", false, ErrStatusInternalServerError
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
		return ErrStatusInternalServerError
	}
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return ErrStatusInternalServerError
	}
	if short == "" {
		logger.Error("в аргементе short нет данных")
		return ErrStatusInternalServerError
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
		return ErrStatusInternalServerError
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

// InternalUserURLsLayerWork слой основной логики для обработчика UserURLs. Возвращается массив пар соответствий и ошибка.
//
// Параметры:
//
//	sl - указателль на ссервис.
func InternalUserURLsLayerWork(sl *ShortLong) ([]txShortURLOriginalURL, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Printf("в аргементе sl, функции internalUserURLsLayerWork, нет указателя")
		return nil, ErrStatusInternalServerError
	}

	sl.muF.muInternalUserURLsLayerWork.Lock()
	defer sl.muF.muInternalUserURLsLayerWork.Unlock()

	// Логика.
	el := txShortURLOriginalURL{}
	shortLong := make([]txShortURLOriginalURL, 0)

	if sl.DB.Ptr != nil { // БД.

		shortLongDB, err := GetAllShortenerDB(sl.DB.Ptr, sl.Log)
		if err != nil {
			sl.Log.Error("Ошибка в функции GetAllShortenerDB",
				zap.Error(err),
			)
			return nil, ErrStatusInternalServerError
		}

		for k, v := range shortLongDB {
			el.ShortURL = sl.BaseAddrShortURL + k
			el.OriginalURL = v

			shortLong = append(shortLong, el)
		}

		if err := ClearShortenerTable(sl.DB.Ptr); err != nil { // Очистка таблицы.
			sl.Log.Error("Ошибка в функции ClearShortenerTable",
				zap.Error(err),
			)
			return nil, ErrStatusInternalServerError
		}

	}

	if sl.DB.Ptr == nil { // Мапы.

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
		return ErrStatusInternalServerError
	}
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return ErrStatusInternalServerError
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
		return ErrStatusInternalServerError
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
		return nil, "", ErrStatusInternalServerError
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return nil, "", ErrStatusInternalServerError
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
		return nil, "", ErrStatusBadRequest
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
		return ErrStatusInternalServerError
	}
	if rxData == nil {
		sl.Log.Error("в аргументе rxData нет указателя")
		return ErrStatusInternalServerError
	}
	if len(rxData) == 0 {
		sl.Log.Error("в аргументе rxData нет данных")
		return ErrStatusBadRequest
	}

	// Логика.
	if err := markFlagDelDB(db, sl, rxData, uuidRx); err != nil {
		sl.Log.Error("Ошибка при обновлении значения флагов daleteFlag",
			zap.Error(err),
		)
		return ErrStatusInternalServerError
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
		return "", ErrStatusInternalServerError
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return "", ErrStatusInternalServerError
	}

	// Логика.
	rxData := r.URL.Path[1:]
	if len(rxData) == 0 {
		return "", ErrStatusBadRequest
	}

	// Возврат.
	return rxData, nil
}

// InternalLongURLFromShortLayerWork слой логики обработчика LongURLFromShort. Возвращется сформированное значение длинного URL и ошибка.
//
// Параметры:
//
//	sl - указатель сервиса.
//	short - принятое сокращённое значение.
func InternalLongURLFromShortLayerWork(sl *ShortLong, short string) (string, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргументе sl, функции internalLongURLFromShortLayerWork, нет указателя")
		return "", ErrStatusInternalServerError
	}
	if short == "" {
		sl.Log.Error("в аргументе rxData нет данных")
		return "", ErrStatusBadRequest
	}

	sl.muF.muInternalLongURLFromShortLayerWork.Lock()
	defer sl.muF.muInternalLongURLFromShortLayerWork.Unlock()

	// Логика.
	var long string
	var err error
	var ok bool

	if sl.DB.Ptr != nil { // БД.

		myErr := fmt.Sprintf("строка с: <%s> не найдена", short)

		long, err = readLongAndFlagByShortDB(sl.DB.Ptr, short)
		if err != nil && err.Error() == myErr {
			return "", ErrStatusNotFound // Если запись в БД нет.
		}
		if err != nil {
			sl.Log.Error("Ошибка в функции readLongAndFlagByShortDB",
				zap.Error(err),
			)
			return "", ErrStatusInternalServerError
		}

		if long == "" {
			return "", ErrStatusGone // Если запись есть, но взведён флаг deleteflag.
		}
	}

	if sl.DB.Ptr == nil { // Мапа.

		long, ok = sl.List.LongByShort[short]
		if !ok {
			sl.Log.Error(fmt.Sprintf("в мапе LongByShort, нет признака существования ключа:<%s>", short))
			return "", ErrStatusBadRequest
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

// --------------------------------
// ---      internalStats       ---
// --------------------------------

// internalStatsLayerWork слой логики обработчика Stats. Возвращется количество сокращённых ссылок, количество пользователй в сервисе и ошибка.
//
// Параметры:
//
//	sl - указатель на конфигурацию сервиса.
func internalStatsLayerWork(sl *ShortLong) (valueURLs, valueUsers int, err error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргументе sl, функции internalStatsLayerWork, нет указателя")
		return 0, 0, ErrStatusInternalServerError
	}
	if sl.Log == nil {
		log.Println("в аргументе sl.Log, функции internalStatsLayerWork, нет указателя")
		return 0, 0, ErrStatusInternalServerError
	}

	// Логика.
	//
	// БД.
	if sl.DB.Ptr != nil {
		valueURLs, err = valueEntriesDB(sl)
		if err != nil {
			sl.Log.Error("ошибка в функции valueEntriesDB", zap.Error(err))
			return 0, 0, ErrStatusInternalServerError

		}
	}

	// in-memory.
	if sl.DB.Ptr == nil {
		valueURLs, err = valueEntriesInMemory(sl)
		if err != nil {
			sl.Log.Error("ошибка в функции valueEntriesInMemory", zap.Error(err))
			return 0, 0, ErrStatusInternalServerError

		}
	}

	// Результат.
	return valueURLs, int(sl.ValueConnect), nil
}

// internalStatsLayerTx слой формироания ответа обраблтчика Stats.
//
// Параметры:
//
//	w - интерфейс ответа.
//	sl - указатель сервиса.
//	valueURLs - количество сокращённых URL.
//	valueUsers - количество пользователй в сервисе.
func internalStatsLayerTx(w http.ResponseWriter, sl *ShortLong, valueURLs, valueUsers int) error {

	// Проверка аргументов.
	if sl == nil {
		log.Println("Ошибка в функции internalStatsLayerTx. В аргументе sl, функции internalStatsLayerWork, нет указателя.")
		return ErrStatusInternalServerError
	}
	if sl.Log == nil {
		log.Println("Ошибка в функции internalStatsLayerTx. В аргументе sl.Log, функции internalStatsLayerWork, нет указателя.")
		return ErrStatusInternalServerError
	}
	if valueURLs < 0 {
		sl.Log.Error("Ошибка в функции internalStatsLayerTx. Значение в аргументе valueURLs, меньше нуля.")
		return ErrStatusInternalServerError
	}
	if valueUsers < 0 {
		sl.Log.Error("Ошибка в функции internalStatsLayerTx. Значение в аргументе valueUsers, меньше нуля.")
		return ErrStatusInternalServerError
	}

	// Логика
	var dataTx txStats

	dataTx.URLs = valueURLs
	dataTx.Users = valueUsers

	byteTx, err := json.Marshal(dataTx)
	if err != nil {
		sl.Log.Error("Ошибка сериализации",
			zap.String("функция", "internalStatsLayerTx"),
			zap.String("err", err.Error()))
		return ErrStatusInternalServerError
	}

	// Передача
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(byteTx)

	return nil
}
