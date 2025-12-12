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
		log.Println("В аргументе logger, функции InternalShortURLFromLongLayerRx, нет указателя.")
		return "", "", ErrNilPointerArgumentLogger
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return "", "", ErrNilPointerArgumentR
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
		return "", "", ErrReadBody
	}
	if len(rxData) == 0 {
		logger.Error("В теле запроса отсутствуют данные",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()))
		return "", "", ErrEmptyDataBody
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
		return "", false, ErrNilPointerArgumentSL
	}
	if longURL == "" {
		sl.Log.Error("в аргументе longURL нет данных")
		return "", false, ErrEmptyArgumentLongURL
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
			return "", flagConflict, ErrFunc
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
		return "", flagConflict, ErrFunc
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
		return ErrNilPointerArgumentW
	}
	if str == "" {
		logger.Error("в аргементе str нет данных")
		return ErrNoContentArgumentStr
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
		return nil, "", ErrNilPointerArgumentLogger
	}
	if r == nil {
		logger.Error("Ошибка в internalShortURLFromLongBatchLayerRx",
			zap.String("причина", "нет указателя на аргументе r"),
		)
		return nil, "", ErrNilPointerArgumentR
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
		return nil, "", ErrReadBody
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
		return nil, "", ErrUnmarshal
	}
	if len(rxLongURLBatch) == 0 {
		logger.Error("Нет данных после десериализации",
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return nil, "", ErrNoContent
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
		log.Println("в аргументе sl, функции internalShortURLFromLongBatchLayerWork, нет указателя")
		return nil, ErrNilPointerArgumentSL
	}
	if longBatch == nil {
		sl.Log.Error("в аргументе longBatch нет указателя")
		return nil, ErrNilPointerLongBatch
	}
	if len(longBatch) == 0 {
		sl.Log.Error("в аргументе longBatch нет данных")
		return nil, ErrNoContentLongBatch
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
			return nil, ErrFunc
		}
	}

	if db == nil { // Мапы.

		err = storageBatchMap(longBatch, sl.List.ShorByLong, sl.List.LongByShort)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в мапы",
				zap.Error(err),
			)
			return nil, ErrFunc
		}

		err = storageFileURL(sl.FileStoragePath, sl.List.ShorByLong, sl.Log)
		if err != nil {
			sl.Log.Error("Ошибка при сохранении в файл",
				zap.Error(err),
			)
			return nil, ErrFunc
		}

		batchShortURL, err = prapareBatchResponse(sl.List.LongByShort, longBatch, sl)
		if err != nil {
			sl.Log.Error("Ошибка при подготовке ответного batch",
				zap.Error(err),
			)
			return nil, ErrFunc
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
		return ErrNilPointerArgumentW
	}
	if shortBatch == nil {
		logger.Error("в аргементе shortBatch нет указателя")
		return ErrNilPointerShortBatch
	}
	if len(shortBatch) == 0 {
		logger.Error("в аргементе shortBatch нет данных")
		return ErrNoContentShortBatch
	}

	// Сериализация.
	txData, err := json.Marshal(shortBatch)
	if err != nil {
		logger.Error("Ошибка при сериализации ответного batch",
			zap.Error(err),
		)
		return ErrMarshal
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
		return RxLongURL{}, "", ErrNilPointerArgumentLogger
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return RxLongURL{}, "", ErrNilPointerArgumentR
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
		return RxLongURL{}, "", ErrReadBody
	}
	if len(rxData) == 0 {
		return RxLongURL{}, "", ErrNoContent
	}

	var rxJSON = RxLongURL{}
	err = json.Unmarshal(rxData, &rxJSON)
	if err != nil {
		logger.Error("Ошибка десереализации",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		return RxLongURL{}, "", ErrUnmarshal
	}
	if rxJSON.URL == "" {
		return RxLongURL{}, "", ErrNoContent
	}

	// Результат.
	uuidRx = r.Header.Get("Authorization")
	rxLong = rxJSON

	return rxLong, uuidRx, nil
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
		return ErrNilPointerArgumentLogger
	}
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return ErrNilPointerArgumentW
	}
	if short == "" {
		logger.Error("в аргементе short нет данных")
		return ErrNoContent
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
		return ErrMarshal
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
		return ErrNilPointerArgumentLogger
	}
	if w == nil {
		logger.Error("в аргементе w нет указателя")
		return ErrNilPointerArgumentW
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
		return fmt.Errorf("ошибка сериализации данных:<%w>", err)
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
		return nil, "", ErrNilPointerArgumentLogger
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return nil, "", ErrNilPointerArgumentR
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
		return nil, "", ErrDecode
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
		return ErrNilPointerArgumentSL
	}
	if rxData == nil {
		sl.Log.Error("в аргументе rxData нет указателя")
		return ErrNoContentArgumentRxData
	}
	if len(rxData) == 0 {
		sl.Log.Error("в аргументе rxData нет данных")
		return ErrNoContent
	}

	// Логика.
	if err := markFlagDelDB(db, sl, rxData, uuidRx); err != nil {
		sl.Log.Error("Ошибка при обновлении значения флагов daleteFlag",
			zap.Error(err),
		)
		return ErrFunc
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
		return "", ErrNilPointerArgumentLogger
	}
	if r == nil {
		logger.Error("в аргументе r нет указателя")
		return "", ErrNilPointerArgumentR
	}

	// Логика.
	rxData := r.URL.Path[1:]
	if len(rxData) == 0 {
		logger.Error("Нет данных короткого представления")
		return "", ErrNoContent
	}

	// Возврат.
	return rxData, nil
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
		return 0, 0, ErrNilPointerArgumentSL
	}
	if sl.Log == nil {
		log.Println("в аргументе sl.Log, функции internalStatsLayerWork, нет указателя")
		return 0, 0, ErrNilPointerArgumentLogger
	}

	// Логика.
	//
	// БД.
	if sl.DB.Ptr != nil {
		valueURLs, err = valueEntriesDB(sl)
		if err != nil {
			sl.Log.Error("ошибка в функции valueEntriesDB", zap.Error(err))
			return 0, 0, fmt.Errorf("ошибка в функции valueEntriesDB:<%w>", err)
		}
	}

	// in-memory.
	if sl.DB.Ptr == nil {
		valueURLs, err = valueEntriesInMemory(sl)
		if err != nil {
			sl.Log.Error("ошибка в функции valueEntriesInMemory", zap.Error(err))
			return 0, 0, fmt.Errorf("ошибка в функции valueEntriesInMemory:<%w>", err)
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
		return ErrNilPointerArgumentSL
	}
	if sl.Log == nil {
		log.Println("Ошибка в функции internalStatsLayerTx. В аргументе sl.Log, функции internalStatsLayerWork, нет указателя.")
		return ErrNilPointerArgumentLogger
	}
	if valueURLs < 0 {
		sl.Log.Error(
			"Ошибка в функции internalStatsLayerTx. Значение в аргументе valueURLs, меньше нуля.",
			zap.Int("значение", valueURLs),
		)
		return fmt.Errorf("значение аргумента valueURLs, меньше нуля:<%d>", valueURLs)
	}
	if valueUsers < 0 {
		sl.Log.Error(
			"Ошибка в функции internalStatsLayerTx. Значение в аргументе valueUsers, меньше нуля.",
			zap.Int("значение", valueUsers),
		)
		return fmt.Errorf("значение аргумента valueUsers, меньше нуля:<%d>", valueUsers)
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
		return fmt.Errorf("ошибка сериализации:<%w>", err)
	}

	// Передача
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(byteTx)

	return nil
}
