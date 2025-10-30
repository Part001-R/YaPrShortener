// service основной пакет приложения.
package service

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	"github.com/go-chi/chi"
	"go.uber.org/zap"
)

type paramsURL struct {
	flags            config.Config
	closeConDB       func()
	storageLongShort handler.Actions
	shortLongDB      *handler.ShortLongDB
}

type checkReasonStop struct {
	chSrvErr     chan error
	chStorageErr chan error
	sigSys       chan os.Signal
	srvConf      *http.Server
	params       *paramsURL
}

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
	err := logger.Initialize(flags.LogLevel)
	if err != nil {
		return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия Initialize вернула ошибку -> <%w>", err)
	}

	// Наблюдатели.
	observer, err := prepareObserver(flags)
	if err != nil {
		return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия prepareObserver вернула ошибку -> <%w>", err)
	}

	// БД.
	var dbPtr *sql.DB
	var funcCloseDB func()

	if flags.DSNDB != "" {

		dbPtr, funcCloseDB, err = db.ConnectDB(flags.DSNDB)
		if err != nil {
			return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия db.ConnectDB вернула ошибку -> <%w>", err)
		}

		err = db.MigrationUpDB(dbPtr)
		if err != nil {
			return &paramsURL{}, fmt.Errorf("ошибка в prepare: функия db.MigrationUpDB вернула ошибку -> <%w>", err)
		}
	}

	// Сервис.
	shortLong := handler.NewShortenerMemory()
	shortLongDB := handler.NewShortenerDB(dbPtr)

	storageLongShort := handler.NewShortener(shortLong, shortLongDB, flags, observer)
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
		return errors.New("ошибка в функции server: в параметре params, нет указателя")
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
func startUpHTTPServer(srv *http.Server, txErr chan error) {

	// Проверка параметров.
	if srv == nil {
		txErr <- errors.New("в параметре srv, нет указателя")
		return
	}
	if txErr == nil {
		log.Fatal("В функции startUpHTTPServer, в параметре txErr, нет указателя на канал")
		return
	}

	logger.Log.Info("Запуск сервера", zap.String("address", srv.Addr))

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logger.Log.Error("Ошибка при запуске сервера", zap.Error(err))
	}
	txErr <- err
}

// signalsStopRun определяет причину остановки выполнения. При штатной остановке, сохраняются метрики.
//
// Параметры:
//
//	data - набор данных для обеспечения работы функции.
func signalsStopRun(data *checkReasonStop) error {

	// Отложенное закрытие базы данных.
	defer func() {
		if data.params.closeConDB != nil { // Приложение может запуститься без подключения к БД.
			data.params.closeConDB()
		}
	}()

	// Проверка аргумента.
	if data == nil {
		return errors.New("ошибка в signalsStopRun: data не инициализирован")
	}
	if data.sigSys == nil {
		return errors.New("ошибка в signalsStopRun: канал sigSys не инициализирован")
	}
	if data.chSrvErr == nil {
		return errors.New("ошибка в signalsStopRun: канал chSrvErr не инициализирован")
	}
	if data.chStorageErr == nil {
		return errors.New("ошибка в signalsStopRun: канал chStorageErr не инициализирован")
	}
	if data.srvConf == nil {
		return errors.New("ошибка в signalsStopRun: srvConf не инициализирована")
	}
	if data.params == nil {
		return errors.New("ошибка в signalsStopRun: params не инициализированы")
	}

	// Логика.
	select {
	case <-data.sigSys:
		logger.Log.Info("сервер остановлен штатно", zap.String("address", data.srvConf.Addr))
		return nil
	case err := <-data.chSrvErr:
		logger.Log.Error("ошибка сервера", zap.String("address", data.srvConf.Addr), zap.String("ошибка", err.Error()))
		return err
	case err := <-data.chStorageErr:
		logger.Log.Error("ошибка периодического сохранения метрик в файл", zap.String("address", data.srvConf.Addr), zap.String("ошибка", err.Error()))
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
		return errors.New("ошибка в функции actions: нет указателя на params")
	}
	if cr == nil {
		return errors.New("ошибка в функции actions: нет указателя на cr")
	}

	srvConf := &http.Server{
		Addr:    params.flags.ServerAddr,
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
	go startUpHTTPServer(srvConf, chSrvErr)

	// Запуск обработчика асинхронной очистки таблицы shortener БД.
	go asynClearShortenerTableDB(params.shortLongDB.Ptr, params.shortLongDB.ChForDelete, params.shortLongDB.ChDoDelete)

	// Приём сигналов остановки.
	err := signalsStopRun(data)
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
//	р - параметры для работы.
func handlersShortener(cr *chi.Mux, p *paramsURL) error {

	// Проверка аргументов.
	if cr == nil {
		return errors.New("ошибка в handlersShortener: в аргументе cr нет указателя")
	}
	if p == nil {
		return errors.New("ошибка в handlersShortener: в аргументе p нет указателя")
	}

	// Без аудита.
	cr.Group(func(r chi.Router) {
		r.Use(p.storageLongShort.Middleware)

		r.Get("/ping", http.HandlerFunc(p.storageLongShort.PingDB))
		r.Post("/api/shorten/batch", http.HandlerFunc(p.storageLongShort.ShortURLFromLongBatch))
		r.Get("/api/user/urls", http.HandlerFunc(p.storageLongShort.UserURLs))
		r.Delete("/api/user/urls", http.HandlerFunc(p.storageLongShort.DeleteUserURLs))
	})

	// Аудит.
	cr.Group(func(r chi.Router) {
		r.Use(p.storageLongShort.MiddlewareAudit)
		r.Use(p.storageLongShort.Middleware)

		r.Post("/", http.HandlerFunc(p.storageLongShort.ShortURLFromLong))
		r.Post("/api/shorten", http.HandlerFunc(p.storageLongShort.ShortURLFromLongJSON))
		r.Get("/{id}", http.HandlerFunc(p.storageLongShort.LongURLFromShort))
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
func asynClearShortenerTableDB(db *sql.DB, rxChForDelete chan handler.DeleteDB, rxChDoDelete chan struct{}) {

	rxMarkData := make([]handler.DeleteDB, 0)

	for {
		select {
		case deleteData, ok := <-rxChForDelete: // Накопление данных для удаления.
			if !ok {
				logger.Log.Error("Ошибка при получении данных из канала. Канал закрыт")
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
						logger.Log.Error("Ошибка при удалении данных",
							zap.Error(err),
							zap.String("short", data.Short),
						)
						continue
					}
					numb, err := result.RowsAffected()
					if err != nil {
						logger.Log.Error("Ошибка при получении номера строки после удаления",
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

				logger.Log.Info("Очистка таблицы shortener завершена",
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
func prepareObserver(flags config.Config) (observer.Action, error) {

	obsSrc := observer.NewObserver()

	// Добавление аудитора - файл.
	if flags.AuditFile != "" {
		name := "file"

		obsFile := observerfile.NewObserverFile(name, flags.AuditFile)
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
