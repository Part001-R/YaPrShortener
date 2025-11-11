// db, содержатся ошибки.
package db

import "errors"

var (
	// Подготовленная ошибка - В DSN нет содержимого.
	ErrEmptyDSN = errors.New("в параметре dsn нет содержимого")

	// Подготовленная ошибка - В log нет указателя.
	ErrNilLog = errors.New("в параметре log нет указателя")

	// Подготовленная ошибка - В DB нет указателя.
	ErrNilDB = errors.New("в параметре db нет указателя")
)
