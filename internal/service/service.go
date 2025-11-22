// service основной пакет приложения.
package service

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Part001-R/YaPrShortener/internal/handler"
	"github.com/Part001-R/YaPrShortener/internal/service/db"
	config "github.com/Part001-R/YaPrShortener/internal/service/flags"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"github.com/Part001-R/YaPrShortener/internal/service/observer/observerfile"
	"github.com/Part001-R/YaPrShortener/internal/service/observer/observerurl"
	"github.com/Part001-R/YaPrShortener/internal/service/sertificat/serthttps"
	"github.com/go-chi/chi"
	"go.uber.org/zap"
)

// Параметры сервиса.
type paramsURL struct {
	flags            config.Config
	closeConDB       func()
	storageLongShort handler.Actions
	shortLongDB      *handler.ShortLongDB
	log              *zap.Logger
}

// Для представления информации по причине остоновки сервиса.
type checkReasonStop struct {
	chSrvErr     chan error
	chStorageErr chan error
	sigSys       chan os.Signal
	srvConf      *http.Server
	params       *paramsURL
}

const (
	namePublicKey  = "publicKey.pem"
	namePrivateKey = "privateKey.pem"
)

// Run содержит подготовительные действия и серверную часть. Возвращает ошибку.
func Run() error {

	// Подготовительные действия
	params, err := prepare()
	if err != nil {
		return fmt.Errorf("функция prepare, вернула ошибку: <%w>", err)
	}

	// Определение ручек и запуск функционала.
	err = server(params)
	if err != nil {
		return fmt.Errorf("функция server, вернула ошибку: <%w>", err)
	}

	return nil
}

// prepare формирует набор параметров, необходимых для работы сервера. Возвращаеются параметры и ошибка.
func prepare() (*paramsURL, error) {

	// Флаги.
	flags := config.ParseFlags()

	// Логгер.
	log, err := logger.NewLogger(flags.LogLevel)
	if err != nil {
		return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия Initialize вернула ошибку -> <%w>", err)
	}

	// Наблюдатели.
	observer, err := prepareObserver(flags, log)
	if err != nil {
		return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия prepareObserver вернула ошибку -> <%w>", err)
	}

	// БД.
	var dbPtr *sql.DB
	var funcCloseDB func()

	if flags.DSNDB != "" {

		dbPtr, funcCloseDB, err = db.ConnectDB(flags.DSNDB, log)
		if err != nil {
			return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия db.ConnectDB вернула ошибку -> <%w>", err)
		}

		err = db.MigrationUpDB(dbPtr, log)
		if err != nil {
			return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия db.MigrationUpDB вернула ошибку -> <%w>", err)
		}
	}

	// Сервис.
	shortLong := handler.NewShortenerMemory()
	shortLongDB := handler.NewShortenerDB(dbPtr)

	storageLongShort := handler.NewShortener(shortLong, shortLongDB, flags, observer, log)
	err = storageLongShort.LoadFileURL()
	if err != nil {
		return &paramsURL{}, fmt.Errorf("ошибка в prepare: функция storageLongShort.LoadFileURL вернула ошибку -> <%w>", err)
	}

	// Результат.
	return &paramsURL{
		flags:            flags,
		closeConDB:       funcCloseDB,
		storageLongShort: storageLongShort,
		shortLongDB:      shortLongDB,
		log:              log,
	}, nil
}

// server содержит основную логику работы сервера. Возвращает ошибку.
//
// Параметры:
//
//	params - параметры необходимые для работы сервера.
func server(params *paramsURL) error {

	// Проверка аргументов.
	if params == nil {
		return ErrNilParams
	}

	cr := chi.NewRouter()

	// Точки входа - Shortener.
	err := handlersShortener(cr, params)
	if err != nil {
		return fmt.Errorf("функция handlersShortener, вернула ошибку: <%w>", err)
	}

	// Действия.
	err = actions(params, cr)
	if err != nil {
		return fmt.Errorf("функция actions, вернула ошибку: <%w>", err)
	}

	return nil
}

// startUpHTTPServer выполняет запуск HTTP сервера.
//
// Парметры:
//
//	srv - настройки сервера.
//	txErr - канал для возврата ошибки.
//	log - логгер.
func startUpHTTPServer(srv *http.Server, txErr chan error, log *zap.Logger) {

	// Проверка параметров.
	if txErr == nil {
		log.Fatal("в функции startUpHTTPServer, в параметре txErr, нет указателя на канал")
	}
	if log == nil {
		txErr <- ErrNilLog
		return
	}
	if srv == nil {
		txErr <- ErrNilSrv
		return
	}

	// Запуск
	log.Info("Запуск HTTP сервера", zap.String("address", srv.Addr))

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Error("Ошибка при запуске сервера", zap.Error(err))
	}
	txErr <- err
}

// startUpHTTPSServer выполняет запуск HTTP сервера.
//
// Парметры:
//
//	srv - настройки сервера.
//	txErr - канал для возврата ошибки.
//	log - логгер.
func startUpHTTPSServer(srv *http.Server, txErr chan error, log *zap.Logger) {

	// Проверка параметров.
	if txErr == nil {
		log.Fatal("в функции startUpHTTPSServer, в параметре txErr, нет указателя на канал")
	}
	if log == nil {
		txErr <- ErrNilLog
		return
	}
	if srv == nil {
		txErr <- ErrNilSrv
		return
	}

	// Проверка существования сертификатов
	dir, err := os.Getwd()
	if err != nil {
		txErr <- fmt.Errorf("ошибка определения рабочей директории: <%v>", err)
	}

	ok, err := serthttps.CheckExistFiles(dir, namePublicKey, namePrivateKey)
	if err != nil {
		txErr <- fmt.Errorf("ошибка при проверку существования сертификатов: <%w>", err)
		return
	}
	if !ok {
		txErr <- errors.New("нет HTTPS сертификатов")
		return
	}

	pathPubKey := dir + "/" + namePublicKey
	pathPrivKey := dir + "/" + namePrivateKey
	// Запуск
	log.Info("Запуск HTTPS сервера", zap.String("address", srv.Addr))

	err = srv.ListenAndServeTLS(pathPubKey, pathPrivKey)
	if err != nil && err != http.ErrServerClosed {
		log.Error("Ошибка при запуске сервера", zap.Error(err))
	}
	txErr <- err
}

// signalsStopRun определяет причину остановки выполнения. При штатной остановке, сохраняются метрики.
//
// Параметры:
//
//	data - набор данных для обеспечения работы функции.
//	log - логгер.
func signalsStopRun(data *checkReasonStop, log *zap.Logger) error {

	// Отложенное закрытие базы данных.
	defer func() {
		if data.params.closeConDB != nil { // Приложение может запуститься без подключения к БД.
			data.params.closeConDB()
		}
	}()

	// Проверка аргументов.
	if log == nil {
		return ErrNilLog
	}
	if data == nil {
		return ErrNilData
	}
	if data.sigSys == nil {
		return ErrNilDataSigSys
	}
	if data.chSrvErr == nil {
		return ErrNilDataChSrvErr
	}
	if data.chStorageErr == nil {
		return ErrNilDataChStorageErr
	}
	if data.srvConf == nil {
		return ErrNilDataSrvConf
	}
	if data.params == nil {
		return ErrNilDataParams
	}

	// Логика.
	select {
	case <-data.sigSys:
		log.Info("сервер остановлен штатно", zap.String("address", data.srvConf.Addr))
		return nil
	case err := <-data.chSrvErr:
		log.Error("ошибка сервера", zap.String("address", data.srvConf.Addr), zap.String("ошибка", err.Error()))
		return err
	case err := <-data.chStorageErr:
		log.Error("ошибка периодического сохранения метрик в файл", zap.String("address", data.srvConf.Addr), zap.String("ошибка", err.Error()))
		return err
	}
}

// actions содержит функциональность сервера. Возвращается ошибка.
//
// Параметры:
//
//	params - параметры для работы сервера.
//	cr - роутер.
func actions(params *paramsURL, cr *chi.Mux) error {

	// Проверка аргументов.
	if params == nil {
		return ErrNilParams
	}
	if params.shortLongDB == nil {
		return ErrNilParamsSrortLongDB
	}
	if params.log == nil {
		return ErrNilParamsLog
	}
	if cr == nil {
		return ErrNilCr
	}

	srvConf := &http.Server{
		Addr:    params.flags.Port,
		Handler: cr,
	}

	// Сигналы остановки.
	chSrvErr := make(chan error)
	chStorageErr := make(chan error)
	sigSys := make(chan os.Signal, 1)

	signal.Notify(sigSys, syscall.SIGINT, syscall.SIGTERM)

	data := &checkReasonStop{
		chSrvErr:     chSrvErr,
		chStorageErr: chStorageErr,
		sigSys:       sigSys,
		srvConf:      srvConf,
		params:       params,
	}

	// Запуск сервера.
	if params.flags.EnableHTTPS == "true" {
		go startUpHTTPSServer(srvConf, chSrvErr, params.log)
	} else {
		go startUpHTTPServer(srvConf, chSrvErr, params.log)
	}

	// Запуск обработчика асинхронной очистки таблицы shortener БД.
	go asynClearShortenerTableDB(params.shortLongDB.Ptr, params.shortLongDB.ChForDelete, params.shortLongDB.ChDoDelete, params.log)

	// Приём сигналов остановки.
	err := signalsStopRun(data, params.log)
	if err != nil {
		return fmt.Errorf("функция signalsStopRun вернула ошибку: <%w>", err)
	}

	return nil
}

// handlersShortener содержит перечень точек входа сервиса сокращения ссылок. Возвращает ошибку.
//
// Параметры:
//
//	cr - роутер.
//	рarams - параметры для работы.
func handlersShortener(cr *chi.Mux, params *paramsURL) error {

	// Проверка аргументов.
	if cr == nil {
		return ErrNilCr
	}
	if params == nil {
		return ErrNilParams
	}

	// Без аудита.
	cr.Group(func(r chi.Router) {
		r.Use(params.storageLongShort.Middleware)

		r.Get("/ping", http.HandlerFunc(params.storageLongShort.PingDB))
		r.Post("/api/shorten/batch", http.HandlerFunc(params.storageLongShort.ShortURLFromLongBatch))
		r.Get("/api/user/urls", http.HandlerFunc(params.storageLongShort.UserURLs))
		r.Delete("/api/user/urls", http.HandlerFunc(params.storageLongShort.DeleteUserURLs))
	})

	// Аудит.
	cr.Group(func(r chi.Router) {
		r.Use(params.storageLongShort.MiddlewareAudit)
		r.Use(params.storageLongShort.Middleware)

		r.Post("/", http.HandlerFunc(params.storageLongShort.ShortURLFromLong))
		r.Post("/api/shorten", http.HandlerFunc(params.storageLongShort.ShortURLFromLongJSON))
		r.Get("/{id}", http.HandlerFunc(params.storageLongShort.LongURLFromShort))
	})

	return nil
}

// asynClearShortenerTableDB реализует асинхронную очистку таблицы shortener БД.
//
// Параметры:
//
//	db - указатель на БД.
//	rxChForDelete - канал для приёма информации по удаляемой строке.
//	rxChDoDelete - канал для приёма признака завершения накопления и запуска очистки таблицы.
//	log - логгер.
func asynClearShortenerTableDB(db *sql.DB, rxChForDelete chan handler.DeleteDB, rxChDoDelete chan struct{}, log *zap.Logger) {

	rxMarkData := make([]handler.DeleteDB, 0)

	for {
		select {
		case deleteData, ok := <-rxChForDelete: // Накопление данных для удаления.
			if !ok {
				log.Error("Ошибка при получении данных из канала. Канал закрыт")
				return
			}
			rxMarkData = append(rxMarkData, deleteData)

		case <-rxChDoDelete: // Приём признака завершения накопления.

			if len(rxMarkData) > 0 {

				cnt := 0
				for _, data := range rxMarkData {

					query := `DELETE FROM shortener WHERE short = $1 AND uuid = $2 AND deleteflag = true`

					result, err := db.Exec(query, data.Short, data.UUID)
					if err != nil {
						log.Error("Ошибка при удалении данных",
							zap.Error(err),
							zap.String("short", data.Short),
						)
						continue
					}
					numb, err := result.RowsAffected()
					if err != nil {
						log.Error("Ошибка при получении номера строки после удаления",
							zap.Error(err),
							zap.String("short", data.Short),
						)
						continue
					}
					if numb > 0 {
						cnt++
					}
				}
				// Очистка накопления.
				rxMarkData = make([]handler.DeleteDB, 0)

				log.Info("Очистка таблицы shortener завершена",
					zap.String("удалено строк", fmt.Sprintf("%d", cnt)),
				)
			}
		}
	}
}

// prepareObserver выполняет подготовку наблюдателей. Возвращается интерфей наблюдателя и ошибка.
//
// Параметры:
//
//	flags - флаги.
//	log - логгер.
func prepareObserver(flags config.Config, log *zap.Logger) (observer.Action, error) {

	// Проверка аргументов.
	if log == nil {
		return nil, ErrNilLog
	}

	// Логика.
	obsSrc := observer.NewObserver(log)

	// Добавление аудитора - файл.
	if flags.AuditFile != "" {
		name := "file"

		obsFile := observerfile.NewObserverFile(name, flags.AuditFile, log)
		obsSrc.RegistrationObserver(obsFile)
	}

	// Добавление аудитора - HTTP.
	if flags.AuditURL != "" {
		name := "http"

		obsURL := observerurl.NewObserverURL(name, flags.AuditURL)
		obsSrc.RegistrationObserver(obsURL)
	}

	return obsSrc, nil
}

// GetValueOrDefault, реализует проверку значения аргумента. Возвращает строку.
//
// Параметры:
//
//	value - анализируемое значение.
func GetValueOrDefault(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}
