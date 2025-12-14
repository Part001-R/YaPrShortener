// handlers, содержатся ошибки.
package handler

import (
	"errors"
)

var (
	/*
		// Подготовленная ошибка - Внутренняя ошибка сервиса.
		ErrStatusInternalServerError = fmt.Errorf("%d", http.StatusInternalServerError)

		// Подготовленная ошибка - Плохой запрос.
		ErrStatusBadRequest = fmt.Errorf("%d", http.StatusBadRequest)

		// Подготовленная ошибка - Не найдено.
		ErrStatusNotFound = fmt.Errorf("%d", http.StatusNotFound)

		// Подготовленная ошибка - Данные отсутствуют.
		ErrStatusGone = fmt.Errorf("%d", http.StatusGone)
	*/

	//===

	// Нет содержимого.
	ErrNoContent = errors.New("нет содержимого")
	// Нет содержимого в аргументе rxData.
	ErrNoContentArgumentRxData = errors.New("в аргументе rxData нет данных")
	// Нет содержимого в аргументе str.
	ErrNoContentArgumentStr = errors.New("в аргументе str нет данных")
	// Нет содержимого в аргументе longBatch.
	ErrNoContentLongBatch = errors.New("в аргументе longBatch нет данных")
	// Нет содержимого в аргументе shortBatch.
	ErrNoContentShortBatch = errors.New("в аргументе shortBatch нет данных")
	// Не найдено.
	ErrNotFound = errors.New("не найдено")
	// Процесс удаления.
	ErrGone = errors.New("удаляется")
	// Ошибка декторирования.
	ErrDecode = errors.New("ошибка декодирования")
	// Ошибка десериализации.
	ErrUnmarshal = errors.New("ошибка десериализации")
	// Ошибка сериализации.
	ErrMarshal = errors.New("ошибка сериализации")
	// Ошибка в функции.
	ErrFunc = errors.New("ошибка функции")
	// Нет указателя в аргументе sl.
	ErrNilPointerArgumentSL = errors.New("в аргументе sl нет указателя")
	// Нет указателя в аргументе logger.
	ErrNilPointerArgumentLogger = errors.New("в аргументе logger нет указателя")
	// Нет указателя в аргументе r.
	ErrNilPointerArgumentR = errors.New("в аргументе r нет указателя")
	// Нет указателя в аргументе w.
	ErrNilPointerArgumentW = errors.New("в аргументе w нет указателя")
	// Нет указателя в аргументе longBatch.
	ErrNilPointerLongBatch = errors.New("в аргументе longBatch нет указателя")
	// Нет указателя в аргументе shortBatch.
	ErrNilPointerShortBatch = errors.New("в аргументе shortBatch нет указателя")
	// В теле запроса отсутствуют данные
	ErrEmptyDataBody = errors.New("в теле запроса отсутствуют данные")
	// В аргументе longURL отсутствуют данные.
	ErrEmptyArgumentLongURL = errors.New("в аргументе longURL нет данных")
	// Ошибка при чтении тела запроса.
	ErrReadBody = errors.New("ошибка при чтении тела запроса")
)
