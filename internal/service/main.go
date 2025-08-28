package service

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/config/config"
	"github.com/Part001-R/YaPrShortener/internal/handler"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"github.com/go-chi/chi"
	"go.uber.org/zap"
)

type paramsURLT struct {
	flags            config.ConfigT
	storageMetrics   handler.MetricsI
	storageLongShort handler.ShortLongI
}

type checkReasonStopT struct {
	chSrvErr     chan error
	chStorageErr chan error
	sigSys       chan os.Signal
	srvConf      *http.Server
	params       *paramsURLT
}

// Функция содержим подготовительные действия и серверную часть. Возвращает ошибку.
func Run() error {

	params, err := prepare()
	if err != nil {
		return fmt.Errorf("функция prepare, вернула ошибку: <%w>", err)
	}

	err = server(params)
	if err != nil {
		return fmt.Errorf("функция server, вернула ошибку: <%w>", err)
	}

	return nil
}

// Функция формирует набор параметров, необходимых для работы сервера. Возвращаеются параметры и ошибка.
func prepare() (*paramsURLT, error) {

	// Флаги
	flags := config.ParseFlags()

	// Логер
	err := logger.Initialize(flags.LogLevel)
	if err != nil {
		return &paramsURLT{}, fmt.Errorf("ошибка в prepare: функия Initialize вернула ошибку -> <%w>", err)
	}

	// Метрики
	metrics := handler.NewMetrics()
	metricsDB := handler.NewMetricsDB(flags.DSNDB)

	storageMetrics := handler.NewMetricsStorage(metrics, metricsDB, flags)

	if flags.RestoreMetr == "true" {
		err := storageMetrics.LoadFileMetrics()
		if err != nil {
			return &paramsURLT{}, fmt.Errorf("ошибка в prepare: функция LoadFileMetrics вернула ошибку -> <%w>", err)
		}
	}

	// Ссылки
	shortLong := handler.NewShortLongURL()
	shortLongDB := handler.NewShortLongURLDB(flags.DSNDB)

	storageLongShort := handler.NewShortLongStorage(shortLong, shortLongDB, flags)
	err = storageLongShort.LoadFileURL()
	if err != nil {
		return &paramsURLT{}, fmt.Errorf("ошибка в prepare: функция LoadFileURL вернула ошибку -> <%w>", err)
	}

	// Результат
	return &paramsURLT{
		flags:            flags,
		storageMetrics:   storageMetrics,
		storageLongShort: storageLongShort,
	}, nil
}

// Функция содержит основную логику работы сервера. Возвращает ошибку.
//
// Параметры:
//
// params - параметры необходимые для работы сервера.
func server(params *paramsURLT) error {

	// Проверка аргументов
	if params == nil {
		return errors.New("ошибка в функции server: в параметре params, нет указателя")
	}

	cr := chi.NewRouter()

	// Точки входа - Shortener
	err := handlersShortener(cr, params)
	if err != nil {
		return fmt.Errorf("функция handlersShortener, вернула ошибку: <%w>", err)
	}

	// Точки входа - Metrics
	err = handlersMetric(cr, params)
	if err != nil {
		return fmt.Errorf("функция handlersMetric, вернула ошибку: <%w>", err)
	}

	// Действия
	err = actions(params, cr)
	if err != nil {
		return fmt.Errorf("функция actions, вернула ошибку: <%w>", err)
	}

	return nil
}

// Функция выполняет запуск HTTP сервера.
//
// Парметры:
//
// srv - настройки сервера.
// txErr - канал для возврата ошибки.
func startUpHTTPServer(srv *http.Server, txErr chan error) {

	// Проверка параметров
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

// Функция выполняет периодическое сохранение метрик в файл.
//
// Параметры:
//
// params - параметры для работы функции.
// txErr - канал для передачи ошибок выполнения.
func periodSaveMetrics(params *paramsURLT, txErr chan error) {

	// Проверка параметров
	if params == nil {
		txErr <- errors.New("ошибка periodSaveMetrics: в параметре params, нет указателя")
		return
	}
	if txErr == nil {
		txErr <- errors.New("ошибка periodSaveMetrics: в параметре txErr, нет указателя")
		return
	}

	// Логика
	if params.flags.StoreIntervalMetr != "0" {

		periodSec, err := strconv.Atoi(params.flags.StoreIntervalMetr)
		if err != nil {
			txErr <- fmt.Errorf("ошибка periodSaveMetrics: ошибка при преобразовании интервала сохранения: <%s>", params.flags.StoreIntervalMetr)
			return
		}

		ticker := time.NewTicker(time.Duration(periodSec) * time.Second)
		defer ticker.Stop()

		// Запуск Go рутины для периодического сохранения в файл
		go func() {
			for range ticker.C {
				if err := params.storageMetrics.StorageMetrics(); err != nil {
					txErr <- fmt.Errorf("функция StorageMetrics вернула ошибку: <%w>", err)
					return
				}
			}
		}()
	}
}

// Функция определяет причину остановки выполнения. При штатной остановке, сохраняются метрики.
//
// Параметры:
//
// data - набор данных для обеспечения работы функции.
func signalsStopRun(data checkReasonStopT) error {

	// Проверка на nil для полей структуры
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

	select {
	case <-data.sigSys:
		err := data.params.storageMetrics.StorageMetrics()
		if err != nil {
			logger.Log.Error("ошибка сохранения метрик при штатном завершении работы", zap.String("ошибка", err.Error()))
		}
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

// Функция содержит функциональность сервера. Возвращается ошибка.
//
// Параметры:
//
// params - параметры для работы сервера.
// cr - мультиплексор.
func actions(params *paramsURLT, cr *chi.Mux) error {

	// Проверка аргументов
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

	// Запуск сервера
	chSrvErr := make(chan error)
	go startUpHTTPServer(srvConf, chSrvErr)

	// Обработка периодического сохранения метрик
	chStorageErr := make(chan error)
	go periodSaveMetrics(params, chStorageErr)

	// Сигналы остановки
	sigSys := make(chan os.Signal, 1)
	signal.Notify(sigSys, syscall.SIGINT, syscall.SIGTERM)

	data := checkReasonStopT{
		chSrvErr:     chSrvErr,
		chStorageErr: chStorageErr,
		sigSys:       sigSys,
		srvConf:      srvConf,
		params:       params,
	}

	err := signalsStopRun(data)
	if err != nil {
		return fmt.Errorf("функция signalsStopRun вернула ошибку: <%w>", err)
	}

	return nil
}

// Функция содержите перечень точек входа для сокращения ссылок. Возвращает ошибку.
//
// Параметры:
//
// cr - мультиплексор.
// р - параметры для работы.
func handlersShortener(cr *chi.Mux, p *paramsURLT) error {

	if cr == nil {
		return errors.New("ошибка в handlersShortener: в аргументе cr нет указателя")
	}
	if p == nil {
		return errors.New("ошибка в handlersShortener: в аргументе p нет указателя")
	}

	cr.Post("/", handler.Middleware(http.HandlerFunc(p.storageLongShort.ShortURLFromLong)))
	cr.Post("/api/shorten", handler.Middleware(http.HandlerFunc(p.storageLongShort.ShortURLFromLongJSON)))
	cr.Get("/{id}", handler.Middleware(http.HandlerFunc(p.storageLongShort.LongURLFromShort)))
	cr.Get("/ping", handler.Middleware(http.HandlerFunc(p.storageLongShort.PingDB)))
	cr.Post("/api/shorten/batch", handler.Middleware(http.HandlerFunc(p.storageLongShort.ShortURLFromLongBatch)))

	cr.Get("/api/user/urls", handler.Middleware(http.HandlerFunc(p.storageLongShort.UserURLs))) //req.Get("/api/user/urls")

	return nil
}

// Функция содержите перечень точек входа для метрик. Возвращает ошибку.
//
// Параметры:
//
// cr - мультиплексор.
// р - параметры для работы.
func handlersMetric(cr *chi.Mux, p *paramsURLT) error {

	if cr == nil {
		return errors.New("ошибка в handlersMetric: в аргументе cr нет указателя")
	}
	if p == nil {
		return errors.New("ошибка в handlersMetric: в аргументе p нет указателя")
	}

	cr.Post("/update/{type}/{name}/{value}", handler.Middleware(http.HandlerFunc(p.storageMetrics.UpdateMetricByTypeAndName)))
	cr.Post("/update", handler.Middleware(http.HandlerFunc(p.storageMetrics.MetricByJSON)))
	cr.Get("/", handler.Middleware(http.HandlerFunc(p.storageMetrics.AllMetricsHTML)))
	cr.Get("/value/{type}/{name}", handler.Middleware(http.HandlerFunc(p.storageMetrics.ValueMetricByTypeAndName)))
	cr.Post("/updates/", handler.Middleware(http.HandlerFunc(p.storageMetrics.UpdateMetricByTypeAndNameBatch)))

	return nil
}
