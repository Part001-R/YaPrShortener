// handlers, содержатся ошибки.
package handler

import (
	"fmt"
	"net/http"
)

var (
	// Подготовленная ошибка - Внутренняя ошибка сервиса.
	ErrStatusInternalServerError = fmt.Errorf("%d", http.StatusInternalServerError)

	// Подготовленная ошибка - Плохой запрос.
	ErrStatusBadRequest = fmt.Errorf("%d", http.StatusBadRequest)

	// Подготовленная ошибка - Не найдено.
	ErrStatusNotFound = fmt.Errorf("%d", http.StatusNotFound)

	// Подготовленная ошибка - Данные отсутствуют.
	ErrStatusGone = fmt.Errorf("%d", http.StatusGone)
)
