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

// internalShortURLFromLong

func InternalShortURLFromLongLayerRx(r *http.Request) (longURL, uuid string, err error) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("в аргементе r нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return "", "", fmt.Errorf("%d", http.StatusInternalServerError)
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
		return "", "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		return "", "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Результат
	uuid = r.Header.Get("Authorization")
	longURL = string(rxData)

	return longURL, uuid, nil
}

func InternalShortURLFromLongLayerWork(db *sql.DB, sl *ShortLongT, longURL, uuidRx string) (result string, flagConflict bool, err error) {

	// Проверка аргументов
	if sl == nil {
		logger.Log.Error("в аргементе sl нет указателя")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if longURL == "" {
		logger.Log.Error("в аргементе longURL нет данных")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // ошибка по уникальности значения длинного представления

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

		// Ответ
		// Конфиликт longURL
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

	// Ответ
	// Конфликта нет
	strResult := sl.BaseAddrShortURL + shortURL

	result = strResult
	flagConflict = false
	return result, flagConflict, nil
}

func InternalShortURLFromLongLayerTx(w http.ResponseWriter, db *sql.DB, str string, flagConflict bool) error {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if str == "" {
		logger.Log.Error("в аргементе str нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	if flagConflict { // Если запись существует
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

// internalShortURLFromLongBatch

func internalShortURLFromLongBatchLayerRx(r *http.Request) (rxLongBatch []rxLongURLBatchT, uuidRx string, err error) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("Ошибка в internalShortURLFromLongBatchLayerRx",
			zap.String("reason", "нет указателя на аргумент r"),
		)
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
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
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Десериализация принятых данных
	rxLongURLBatch := make([]rxLongURLBatchT, 0)

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

	// Возврат
	rxLongBatch = rxLongURLBatch
	uuidRx = r.Header.Get("Authorization")
	return rxLongBatch, uuidRx, nil

}

func internalShortURLFromLongBatchLayerWork(db *sql.DB, sl *ShortLongT, longBatch []rxLongURLBatchT, uuidRx string) ([]txShortURLBatchT, error) {

	// Проверка аргументов
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

	// Логика
	batchShortURL := make([]txShortURLBatchT, 0)
	var err error

	if db != nil { // БД

		batchShortURL, err = allActionsStorageBatchDBURL(db, longBatch, sl.BaseAddrShortURL, uuidRx)
		if err != nil {
			logger.Log.Error("Ошибка при сохранении в БД",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}
	}

	if db == nil { // Мапы

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

	// Результат
	return batchShortURL, nil
}

func internalShortURLFromLongBatchLayerTx(w http.ResponseWriter, shortBatch []txShortURLBatchT) error {

	// Проверка аргументов
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

	// Сериализация
	txData, err := json.Marshal(shortBatch)
	if err != nil {
		logger.Log.Error("Ошибка при сериализации ответного batch",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(txData)

	return nil
}

// internalShortURLFromLongJSON

func internalShortURLFromLongJSONLayerRx(r *http.Request) (rxLong rxLongURLT, uuidRx string, err error) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
		return rxLongURLT{}, "", fmt.Errorf("%d", http.StatusInternalServerError)
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
		return rxLongURLT{}, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		return rxLongURLT{}, "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	var rxJSON = rxLongURLT{}
	err = json.Unmarshal(rxData, &rxJSON)
	if err != nil {
		return rxLongURLT{}, "", fmt.Errorf("%d", http.StatusBadRequest)
	}
	if rxJSON.URL == "" {
		return rxLongURLT{}, "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Результат
	uuidRx = r.Header.Get("Authorization")
	rxLong = rxJSON

	return rxLong, uuidRx, nil
}

func internalShortURLFromLongJSONLayerWork(db *sql.DB, sl *ShortLongT, rxJSON rxLongURLT, uuidRx string) (short string, flagConflict bool, err error) {

	// Проверка аргументов
	if sl == nil {
		logger.Log.Error("в аргементе sl нет указателя")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if rxJSON.URL == "" {
		logger.Log.Error("в аргементе rxJSON.URL нет данных")
		return "", false, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	errUniqueLong := `pq: duplicate key value violates unique constraint "idx_shortener_long"` // ошибка по уникальности значения длинного представления

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

		// Ответ
		flagConflict = true
		short = sl.BaseAddrShortURL + shortURL
		return short, flagConflict, nil
	}

	// Ответ
	flagConflict = false
	short = sl.BaseAddrShortURL + shortURL
	return short, flagConflict, nil
}

func internalShortURLFromLongJSONLayerTx(w http.ResponseWriter, short string, flagConflict bool) error {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if short == "" {
		logger.Log.Error("в аргементе short нет данных")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	var txJSON = txShortURLT{
		Result: short,
	}
	txData, err := json.Marshal(txJSON)
	if err != nil {
		logger.Log.Error("Ошибка сериализации данных",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Ответ
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

// internalUserURLs

func internalUserURLsLayerWork(db *sql.DB, sl *ShortLongT) ([]txShortURLOriginalURLT, error) {

	// Проверка аргументов
	if sl == nil {
		logger.Log.Error("в аргементе sl нет указателя")
		return nil, fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	el := txShortURLOriginalURLT{}
	shortLong := make([]txShortURLOriginalURLT, 0)

	if db != nil { // БД

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

		if err := ClearShortenerTable(db); err != nil { // очистка таблицы
			logger.Log.Error("Ошибка в функции ClearShortenerTable",
				zap.Error(err),
			)
			return nil, fmt.Errorf("%d", http.StatusInternalServerError)
		}

	}

	if db == nil { // Мапы

		for k, v := range sl.List.LongByShort {
			el.ShortURL = sl.BaseAddrShortURL + k
			el.OriginalURL = v

			shortLong = append(shortLong, el)
		}

		sl.List.LongByShort = make(map[string]string) // очистка мапы
	}

	// Результат
	return shortLong, nil
}

func internalUserURLsLayerTx(w http.ResponseWriter, shortLong []txShortURLOriginalURLT) error {

	// Проверка аргументов
	if w == nil {
		logger.Log.Error("в аргементе w нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	w.Header().Set("Content-Type", "application/json")

	if len(shortLong) == 0 {

		// Ответ
		uuid := authoriz.GenerateUniqueID()
		authoriz.SetUserCookie(w, uuid)

		authoriz.UUID = uuid // ?

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

	// Ответ
	w.WriteHeader(http.StatusOK)
	w.Write(txData)

	return nil
}

// internalDeleteUserURLs

func internalDeleteUserURLsLayerRx(r *http.Request) (rxArr []string, uuidRx string, err error) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	uuidRx = r.Header.Get("Authorization")

	rxByteBody, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.Error("Ошибка чтения тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", fmt.Errorf("%d", http.StatusInternalServerError)
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
		return nil, "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Ответ
	rxArr = rxArray
	return rxArr, uuidRx, nil
}

func internalDeleteUserURLsLayerWork(db *sql.DB, sl *ShortLongT, rxData []string, uuidRx string) error {

	// Проверка аргументов
	if rxData == nil {
		logger.Log.Error("в аргументе rxData нет указателя")
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if len(rxData) == 0 {
		logger.Log.Error("в аргументе rxData нет данных")
		return fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Логика
	if err := markFlagDelDB(db, sl, rxData, uuidRx); err != nil {
		logger.Log.Error("Ошибка при обновлении значения флагов daleteFlag",
			zap.Error(err),
		)
		return fmt.Errorf("%d", http.StatusInternalServerError)
	}

	return nil
}

func internalDeleteUserURLsLayerTx(w http.ResponseWriter) {

	w.WriteHeader(http.StatusAccepted)
}

// internalLongURLFromShort

func internalLongURLFromShortLayerRx(r *http.Request) (string, error) {

	// Проверка аргументов
	if r == nil {
		logger.Log.Error("в аргументе r нет указателя")
		return "", fmt.Errorf("%d", http.StatusInternalServerError)
	}

	// Логика
	rxData := r.URL.Path[1:]
	if len(rxData) == 0 {
		return "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Возврат
	return rxData, nil
}

func internalLongURLFromShortLayerWork(db *sql.DB, sl *ShortLongT, short string) (string, error) {

	// Проверка аргументов
	if sl == nil {
		logger.Log.Error("в аргументе sl нет указателя")
		return "", fmt.Errorf("%d", http.StatusInternalServerError)
	}
	if short == "" {
		logger.Log.Error("в аргументе rxData нет данных")
		return "", fmt.Errorf("%d", http.StatusBadRequest)
	}

	// Логика
	var long string
	var err error
	var ok bool

	if db != nil { // БД

		myErr := fmt.Sprintf("строка с: <%s> не найдена", short)

		long, err = readLongAndFlagByShortDB(db, short)
		if err != nil && err.Error() == myErr {
			return "", fmt.Errorf("%d", http.StatusNotFound) // Если запись в БД нет
		}
		if err != nil {
			logger.Log.Error("Ошибка в функции readLongAndFlagByShortDB",
				zap.Error(err),
			)
			return "", fmt.Errorf("%d", http.StatusInternalServerError)
		}

		if long == "" {
			return "", fmt.Errorf("%d", http.StatusGone) // Если запись есть, но взведён флаг deleteflag
		}
	}

	if db == nil { // Мапа

		long, ok = sl.List.LongByShort[short]
		if !ok {
			logger.Log.Error("в аргументе rxData нет данных")
			return "", fmt.Errorf("%d", http.StatusBadRequest)
		}
		long = strings.Trim(long, "\"")
	}

	// Возврат
	return long, nil
}

func internalLongURLFromShortLayerTx(w http.ResponseWriter, long string) {

	w.Header().Set("Location", long)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
