package handler

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var onceInit sync.Once

type internWorkFunc struct{}

type internalWork interface {
	InternalShortURLFromLongJSONLayerWork(sl *ShortLong, rxJSON RxLongURL, uuidRx string) (short string, flagConflict bool, err error)
	InternalLongURLFromShortLayerWork(sl *ShortLong, short string) (string, error)
	InternalUserURLsLayerWork(sl *ShortLong) ([]txShortURLOriginalURL, error)
}

// Интерфейс.
type ActionsWorkFunc interface {
	internalWork
}

var workFunc *internWorkFunc

// Конструктор.
func NewInstWorkFunc() ActionsWorkFunc {
	onceInit.Do(func() {
		workFunc = &internWorkFunc{}
	})
	return workFunc
}

// InternalShortURLFromLongJSONLayerWork слой основной логики для обработчика ShortURLFromLongJSON. Возвращается короткое представление URL, флаг конфликта и ошибка.
//
// Параметры:
//
//	sl - указатель на сервис.
//	rxJSON - принятое значение длинного URL.
//	uuidRx - принятый ID.
func (i internWorkFunc) InternalShortURLFromLongJSONLayerWork(sl *ShortLong, rxJSON RxLongURL, uuidRx string) (short string, flagConflict bool, err error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргументе sl, функции internalShortURLFromLongJSONLayerWork, нет указателя")
		return "", false, ErrNilPointerArgumentSL
	}
	if rxJSON.URL == "" {
		sl.Log.Error("в аргументе rxJSON.URL нет данных")
		return "", false, ErrNoContent
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
			return "", false, ErrFunc
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

// InternalLongURLFromShortLayerWork слой логики обработчика LongURLFromShort. Возвращется сформированное значение длинного URL и ошибка.
//
// Параметры:
//
//	sl - указатель сервиса.
//	short - принятое сокращённое значение.
func (i internWorkFunc) InternalLongURLFromShortLayerWork(sl *ShortLong, short string) (string, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Println("в аргументе sl, функции internalLongURLFromShortLayerWork, нет указателя")
		return "", ErrNilPointerArgumentSL
	}
	if short == "" {
		sl.Log.Error("в аргументе rxData нет данных")
		return "", ErrNoContentArgumentRxData
	}

	sl.muF.muInternalLongURLFromShortLayerWork.Lock()
	defer sl.muF.muInternalLongURLFromShortLayerWork.Unlock()

	// Логика.
	var long string
	var err error
	var ok bool

	if sl.DB.Ptr != nil { // БД.

		myErr := fmt.Errorf("строка с: <%s> не найдена", short)

		long, err = readLongAndFlagByShortDB(sl.DB.Ptr, short)
		if err != nil && errors.Is(err, myErr) {
			return "", ErrNotFound
		}
		if err != nil {
			sl.Log.Error("Ошибка в функции readLongAndFlagByShortDB",
				zap.Error(err),
			)
			return "", ErrFunc
		}

		if long == "" {
			return "", ErrGone // Если запись есть, но взведён флаг deleteflag.
		}
	}

	if sl.DB.Ptr == nil { // Мапа.

		long, ok = sl.List.LongByShort[short]
		if !ok {
			sl.Log.Error("в мапе LongByShort, нет признака существования ключа",
				zap.String("ключ", short),
			)
			return "", ErrNoContent
		}
		long = strings.Trim(long, "\"")
	}

	// Возврат.
	return long, nil
}

// InternalUserURLsLayerWork слой основной логики для обработчика UserURLs. Возвращается массив пар соответствий и ошибка.
//
// Параметры:
//
//	sl - указателль на ссервис.
func (i internWorkFunc) InternalUserURLsLayerWork(sl *ShortLong) ([]txShortURLOriginalURL, error) {

	// Проверка аргументов.
	if sl == nil {
		log.Printf("в аргементе sl, функции internalUserURLsLayerWork, нет указателя")
		return nil, ErrNilPointerArgumentSL
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
			return nil, fmt.Errorf("ошибка в функции GetAllShortenerDB:<%w>", err)
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
			return nil, fmt.Errorf("ошибка в функции ClearShortenerTable:<%w>", err)
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
