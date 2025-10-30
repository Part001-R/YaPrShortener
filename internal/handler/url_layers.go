// handler пакет. Секция слоёв обработчиков контроллера.
package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Part001-R/YaPrShortener/internal/service/authoriz"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
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
func InternalShortURLFromLongLayerRx(r *http.Request) (longURL, uuid string, err error) {

	// Проверка аргументов.
	if r == nil {
		logger.Log.Error("в аргементе r нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return "", "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Чтение тела запроса.
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
		logger.Log.Error("в аргементе sl нет указателя")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if longURL == "" {
		logger.Log.Error("в аргементе longURL нет данных")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // Ошибка по уникальности значения длинного представления.

	shortURL, err := workWithRxData(db, sl, longURL, uuidRx)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(db, longURL)
		if err != nil {
			logger.Log.Error("Ошибка при получении короткого представления по длинному URL",
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
		logger.Log.Error("Ошибка в функции workWithRxData",
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
func InternalShortURLFromLongLayerTx(w http.ResponseWriter, str string, flagConflict bool) error {

	// Проверка аргументов.
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if str == "" {
		logger.Log.Error("в аргементе str нет данных")
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
func internalShortURLFromLongBatchLayerRx(r *http.Request) (rxLongBatch []rxLongURLBatch, uuidRx string, err error) {

	// Проверка аргументов.
	if r == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongBatchLayerRx",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Чтение тела запроса.
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
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Десериализация принятых данных.
	rxLongURLBatch := make([]rxLongURLBatch, 0)

	err = json.Unmarshal(rxData, &rxLongURLBatch)
	if err != nil {
		logger.Log.Error("Ошибка десериализации",
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
		logger.Log.Error("в аргементе sl нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if longBatch == nil {
		logger.Log.Error("в аргементе longBatch нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(longBatch) == 0 {
		logger.Log.Error("в аргементе longBatch нет данных")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	batchShortURL := make([]txShortURLBatch, 0)
	var err error

	if db != nil { // БД.

		batchShortURL, err = allActionsStorageBatchDBURL(db, longBatch, sl.BaseAddrShortURL, uuidRx)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в БД",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}
	}

	if db == nil { // Мапы.

		err = storageBatchMap(longBatch, sl.List.ShorByLong, sl.List.LongByShort)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в мапы",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в файл",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

		batchShortURL, err = prapareBatchResponse(sl.List.LongByShort, longBatch, sl)
		if err != nil {
			logger.Log.Error("Ошибка при подготовке ответного batch",
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
func internalShortURLFromLongBatchLayerTx(w http.ResponseWriter, shortBatch []txShortURLBatch) error {

	// Проверка аргументов.
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if shortBatch == nil {
		logger.Log.Error("в аргементе shortBatch нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(shortBatch) == 0 {
		logger.Log.Error("в аргементе shortBatch нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Сериализация.
	txData, err := json.Marshal(shortBatch)
	if err != nil {
		logger.Log.Error("Ошибка при сериализации ответного batch",
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
func internalShortURLFromLongJSONLayerRx(r *http.Request) (rxLong rxLongURL, uuidRx string, err error) {

	// Проверка аргументов.
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
		return rxLongURL{}, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
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
		logger.Log.Error("в аргементе sl нет указателя")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if rxJSON.URL == "" {
		logger.Log.Error("в аргементе rxJSON.URL нет данных")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // Ошибка по уникальности значения длинного представления.

	shortURL, err := workWithRxData(db, sl, rxJSON.URL, uuidRx)
	if err != nil && errors.Unwrap(err).Error() == errUniqueLong {

		shortURL, err = readShortByLongDB(db, rxJSON.URL)
		if err != nil {
			logger.Log.Error("Ошибка в функции readShortByLongDB",
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
func internalShortURLFromLongJSONLayerTx(w http.ResponseWriter, short string, flagConflict bool) error {

	// Проверка аргументов.
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if short == "" {
		logger.Log.Error("в аргементе short нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	var txJSON = txShortURL{
		Result: short,
	}
	txData, err := json.Marshal(txJSON)
	if err != nil {
		logger.Log.Error("Ошибка сериализации данных",
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
		logger.Log.Error("в аргементе sl нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	el := txShortURLOriginalURL{}
	shortLong := make([]txShortURLOriginalURL, 0)

	if db != nil { // БД.

		shortLongDB, err := GetAllShortenerDB(db)
		if err != nil {
			logger.Log.Error("Ошибка в функции GetAllShortenerDB",
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
			logger.Log.Error("Ошибка в функции ClearShortenerTable",
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
func internalUserURLsLayerTx(w http.ResponseWriter, shortLong []txShortURLOriginalURL) error {

	// Проверка аргументов.
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
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
		logger.Log.Error("Ошибка сериализации ответа",
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
func internalDeleteUserURLsLayerRx(r *http.Request) (rxArr []string, uuidRx string, err error) {

	// Проверка аргументов.
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика.
	uuidRx = r.Header.Get("Authorization")

	// Используем json.Decoder для поточной разбора.
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&rxArr); err != nil {
		logger.Log.Error("Ошибка сериализации данных",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusBadRequest)
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
	if rxData == nil {
		logger.Log.Error("в аргументе rxData нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		logger.Log.Error("в аргументе rxData нет данных")
		return fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Логика.
	if err := markFlagDelDB(db, sl, rxData, uuidRx); err != nil {
		logger.Log.Error("Ошибка при обновлении значения флагов daleteFlag",
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
func internalLongURLFromShortLayerRx(r *http.Request) (string, error) {

	// Проверка аргументов.
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
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
		logger.Log.Error("в аргументе sl нет указателя")
		return "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if short == "" {
		logger.Log.Error("в аргументе rxData нет данных")
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
			logger.Log.Error("Ошибка в функции readLongAndFlagByShortDB",
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
			logger.Log.Error("в аргументе rxData нет данных")
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
