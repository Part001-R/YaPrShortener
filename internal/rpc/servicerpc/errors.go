package servicerpc

import "errors"

const (
	missingMetadata      = "отсутствуют метаданные"
	missingAuthorization = "нет данных в authorization"
	alreadyExixts        = "конфликт: Запись уже существует"
	internalServerErr    = "внутренняя ошибка сервера"
	badRequest           = "плохой запрос"
	dataUnavalible       = "данные не доступны"
	serverUnavalible     = "сервер временно не доступен"
)

var (
	errMissingMetadata      = errors.New(missingMetadata)
	errMissingAuthorization = errors.New(missingAuthorization)
	errAlreadyExixts        = errors.New(alreadyExixts)
)
