// service, содержатся ошибки.
package service

import "errors"

var (
	// Подготовленная ошибка - в параметре params, нет указателя.
	ErrNilParams = errors.New("в параметре params, нет указателя")

	// Подготовленная ошибка - в параметре log, нет указателя.
	ErrNilLog = errors.New("в параметре log, нет указателя")

	// Подготовленная ошибка - в параметре srv, нет указателя.
	ErrNilSrv = errors.New("в параметре srv, нет указателя")

	// Подготовленная ошибка - в параметре data, нет указателя.
	ErrNilData = errors.New("в параметре data, нет указателя")

	// Подготовленная ошибка - канал data.sigSys не инициализирован.
	ErrNilDataSigSys = errors.New("канал data.sigSys не инициализирован")

	// Подготовленная ошибка - канал data.chSrvErr не инициализирован.
	ErrNilDataChSrvErr = errors.New("канал data.chSrvErr не инициализирован")

	// Подготовленная ошибка - канал data.chStorageErr не инициализирован.
	ErrNilDataChStorageErr = errors.New("канал data.chStorageErr не инициализирован")

	// Подготовленная ошибка - data.srvConf нет инициализации.
	ErrNilDataSrvConf = errors.New("data.srvConf нет инициализации")

	// Подготовленная ошибка - data.params нет инициализации.
	ErrNilDataParams = errors.New("data.params нет инициализации")

	// Подготовленная ошибка - в параметре cr, нет указателя.
	ErrNilCr = errors.New("в параметре cr, нет указателя")

	// Подготовленная ошибка - в параметре params.shortLongDB, нет указателя.
	ErrNilParamsSrortLongDB = errors.New("в параметре params.shortLongDB, нет указателя")

	// Подготовленная ошибка - в параметре params.log, нет указателя.
	ErrNilParamsLog = errors.New("в параметре params.log, нет указателя")
)
