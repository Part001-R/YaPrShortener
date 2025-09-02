package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/config/config"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"go.uber.org/zap"
)

type MetricsT struct {
	GaugeMetrics   map[string]float64
	CounterMetrics map[string]int64
	Mu             sync.RWMutex
}

type MetricsDBT struct {
	ptr *sql.DB
	Mu  sync.RWMutex
}

type MetricsHandlerT struct {
	Metrics             *MetricsT
	DB                  *MetricsDBT
	StoreIntervalMetr   string
	FileStoragePathMetr string
	RestoreMetr         string
}

type Metrics struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta *int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value *float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

type EventMetricT struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type rxMetricsBatchT struct {
	TypeM  string `json:"type_m"`
	NameM  string `json:"name_m"`
	ValueM string `json:"value_m"`
}

type mInterfaceFileI interface {
	LoadFileMetrics() error
	StorageMetrics() error
}

type mInterfaceHTMLI interface {
	AllMetricsHTML(w http.ResponseWriter, r *http.Request)
}

type mInterfaceMetricI interface {
	UpdateMetricByTypeAndName(w http.ResponseWriter, r *http.Request)
	ValueMetricByTypeAndName(w http.ResponseWriter, r *http.Request)
	MetricByJSON(w http.ResponseWriter, r *http.Request)
	UpdateMetricByTypeAndNameBatch(w http.ResponseWriter, r *http.Request)
}

type MetricsI interface {
	mInterfaceFileI
	mInterfaceHTMLI
	mInterfaceMetricI
}

func NewMetricsStorage(m *MetricsT, db *MetricsDBT, f config.ConfigT) MetricsI {
	return &MetricsHandlerT{
		Metrics:             m,
		DB:                  db,
		StoreIntervalMetr:   f.StoreIntervalMetr,
		FileStoragePathMetr: f.FileStoragePathMetr,
		RestoreMetr:         f.RestoreMetr,
	}
}

func NewMetricsDB(db *sql.DB) *MetricsDBT {
	return &MetricsDBT{
		ptr: db,
		Mu:  sync.RWMutex{},
	}
}

func NewMetrics() *MetricsT {
	return &MetricsT{
		GaugeMetrics:   make(map[string]float64),
		CounterMetrics: make(map[string]int64),
		Mu:             sync.RWMutex{},
	}
}

func (m *MetricsHandlerT) UpdateMetricByTypeAndName(w http.ResponseWriter, r *http.Request) {

	m.Metrics.Mu.RLock()
	defer m.Metrics.Mu.RUnlock()

	internalUpdateMetricByTypeAndName(m.DB.ptr, m, w, r)
}

func (m *MetricsHandlerT) UpdateMetricByTypeAndNameBatch(w http.ResponseWriter, r *http.Request) {

	m.DB.Mu.RLock()
	defer m.DB.Mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain")

	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение тела запроса
	rxData, err := io.ReadAll(r.Body)
	defer func() {
		err = r.Body.Close()
		logger.Log.Error("Ошибка при закрытии r.Body",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
	}()
	if err != nil {
		logger.Log.Error("Ошибка при чтении тела запроса",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Десериализация принятых данных
	rxMetricsBatch := make([]rxMetricsBatchT, 0)

	err = json.Unmarshal(rxData, &rxMetricsBatch)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if len(rxMetricsBatch) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Обработка
	if m.DB.ptr != nil { // сохранение пары соответствия в БД

		err = allActionsStorageBatchDBMetricsTx(m.DB.ptr, rxMetricsBatch)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	if m.DB.ptr == nil { // сохранение пары соответствия в мапы и файл

		// Сохранение в мапы
		err := storageMetricsInMap(rxMetricsBatch, m.Metrics.GaugeMetrics, m.Metrics.CounterMetrics)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		// Синхронное сохранение в файл
		if m.StoreIntervalMetr == "0" {
			err := storage(m.FileStoragePathMetr, m.Metrics.GaugeMetrics, m.Metrics.CounterMetrics)
			if err != nil {
				logger.Log.Error("Ошибка при синхронном сохранении в файл",
					zap.Error(err),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusCreated)
}

func (m *MetricsHandlerT) MetricByJSON(w http.ResponseWriter, r *http.Request) {
	m.Metrics.Mu.RLock()
	defer m.Metrics.Mu.RUnlock()

	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if r.Header.Get("Content-Type") != `application/json` {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var rxData Metrics
	err := json.NewDecoder(r.Body).Decode(&rxData)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if rxData.ID == "" || rxData.MType == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rawData := Metrics{}

	switch rxData.MType {
	case "counter":
		if _, exists := m.Metrics.CounterMetrics[rxData.ID]; !exists {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		value := m.Metrics.CounterMetrics[rxData.ID]
		rawData.Delta = &value

	case "gauge":
		if _, exists := m.Metrics.GaugeMetrics[rxData.ID]; !exists {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		value := m.Metrics.GaugeMetrics[rxData.ID]
		rawData.Value = &value

	default:
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	rawData.ID = rxData.ID
	rawData.MType = rxData.MType

	txData, err := json.Marshal(rawData)
	if err != nil {
		logger.Log.Error("Ошибка сериализации данных",
			zap.Error(err),
			zap.String("method", r.Method),
			zap.String("url", r.URL.String()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(txData)
}

func (m *MetricsHandlerT) AllMetricsHTML(w http.ResponseWriter, r *http.Request) {
	m.Metrics.Mu.RLock()
	defer m.Metrics.Mu.RUnlock()

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintln(w, "<html><head><title>МЕТРИКИ</title></head><body>")
	fmt.Fprintln(w, "<h1>Доступные метрики</h1>")

	// Gauge метрики
	fmt.Fprintln(w, "<h2>Gauge</h2><ul>")
	gaugeKeys := make([]string, 0, len(m.Metrics.GaugeMetrics))
	for key := range m.Metrics.GaugeMetrics {
		gaugeKeys = append(gaugeKeys, key)
	}
	sort.Strings(gaugeKeys)
	for _, key := range gaugeKeys {
		fmt.Fprintf(w, "<li>%s: %f</li>\n", key, m.Metrics.GaugeMetrics[key])
	}
	fmt.Fprintln(w, "</ul>")

	// Counter метрики
	fmt.Fprintln(w, "<h2>Counter</h2><ul>")
	counterKeys := make([]string, 0, len(m.Metrics.CounterMetrics))
	for key := range m.Metrics.CounterMetrics {
		counterKeys = append(counterKeys, key)
	}
	sort.Strings(counterKeys)
	for _, key := range counterKeys {
		fmt.Fprintf(w, "<li>%s: %d</li>\n", key, m.Metrics.CounterMetrics[key])
	}
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "</body></html>")
}

func (m *MetricsHandlerT) ValueMetricByTypeAndName(w http.ResponseWriter, r *http.Request) {

	m.Metrics.Mu.RLock()
	defer m.Metrics.Mu.RUnlock()

	internalValueMetricByTypeAndName(m, w, r)

}

func (m *MetricsHandlerT) StorageMetrics() error {

	m.Metrics.Mu.RLock()
	defer m.Metrics.Mu.RUnlock()

	err := storage(m.FileStoragePathMetr, m.Metrics.GaugeMetrics, m.Metrics.CounterMetrics)
	if err != nil {
		return fmt.Errorf("ошибка сохранения значений метрик в файл <%w>", err)
	}

	return nil
}

func (m *MetricsHandlerT) LoadFileMetrics() error {

	m.Metrics.Mu.RLock()
	defer m.Metrics.Mu.RUnlock()

	// Проверка
	if m.FileStoragePathMetr == "" {
		return errors.New("не указан путь к файлу с метриками")
	}
	if m.Metrics.CounterMetrics == nil {
		return fmt.Errorf("нет указателя на мапу метрик counter")
	}
	if m.Metrics.GaugeMetrics == nil {
		return fmt.Errorf("нет указателя на мапу метрик counter")
	}

	// Файл
	file, err := os.OpenFile(m.FileStoragePathMetr, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла <%s>: %v", m.FileStoragePathMetr, err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("ошибка получения статуса файла: %v", err)
	}
	if fi.Size() == 0 {
		return nil
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла: %v", err)
	}

	var events []EventMetricT
	if err := json.Unmarshal(data, &events); err != nil {
		return fmt.Errorf("ошибка Unmarshal: %v", err)
	}

	// Сохранение данных из файла в мапы
	for _, ev := range events {

		if ev.Type == "counter" {
			value, err := strconv.ParseInt(ev.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("ошибка парсинга <%s>, c значением <%s>", ev.ID, ev.Value)
			}
			m.Metrics.CounterMetrics[ev.ID] = value
		}

		if ev.Type == "gauge" {
			value, err := strconv.ParseFloat(ev.Value, 64)
			if err != nil {
				return fmt.Errorf("ошибка парсинга <%s>, c значением <%s>", ev.ID, ev.Value)
			}
			m.Metrics.GaugeMetrics[ev.ID] = value
		}
	}
	return nil
}

func storage(filePath string, gM map[string]float64, cM map[string]int64) error {

	// Проверка аргументов
	if filePath == "" {
		return errors.New("не указан путь к файлу хранения метрик")
	}
	if gM == nil {
		return errors.New("нет указателя на мапу gauge метрик")
	}
	if cM == nil {
		return errors.New("нет указателя на мапу counter метрик")
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("ошибка <%v> открытия файла <%s>", err, filePath)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Log.Error("Ошибка закрытия подключения к файлу",
				zap.Error(err),
			)
		}

	}()

	if len(gM) == 0 && len(cM) == 0 {
		return nil
	}

	// Вывод с построением строк
	file.WriteString("[\n")

	// Обработка gauge
	if len(gM) != 0 {
		numb := 1
		size := len(gM)
		for k, v := range gM {

			str := fmt.Sprintf(`	{"id":"%s","type":"%s","value":"%.f"}`, k, "gauge", v)
			file.WriteString(str)
			if numb < size {
				file.WriteString(",\n")
			}
			numb++
		}
	}
	// Обработка counter
	if len(cM) != 0 {

		if len(gM) > 0 {
			file.WriteString(",\n")
		}

		numb := 1
		size := len(cM)
		for k, v := range cM {

			str := fmt.Sprintf(`	{"id":"%s","type":"%s","delta":"%d"}`, k, "counter", v)
			file.WriteString(str)
			if numb < size {
				file.WriteString(",\n")
			}
			numb++
		}
	}

	file.WriteString("\n")
	file.WriteString("]\n")

	return nil
}

// Функция выполняет сохранение в БД метрик типа counter. Возвращает ошибку.
//
// Параметры:
//
// db - указатель на БД.
// name - имя метрики.
// value - значение метрики.
func storageDBCounterMetrics(db *sql.DB, name string, value int64) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("ошибка сохранения метрики типа counter в БД. В аргументе db нет указателя на БД")
	}
	if name == "" {
		return errors.New("ошибка сохранения метрики типа counter в БД. Принято пустое значение name аргумента")
	}

	// Сохранение (обновление) метрики
	str := `
		INSERT INTO counters (name_m, value_m) 
		VALUES ($1, $2) 
		ON CONFLICT (name_m) DO UPDATE 
		SET value_m = EXCLUDED.value_m, 
    	created_at = CURRENT_TIMESTAMP;
		`

	result, err := db.Exec(str, name, value)
	if err != nil {
		return fmt.Errorf("ошибка сохранения метрики типа counter в БД. Не удалось сохранить метрику:<%s> с его значением:<%d>", name, value)
	}
	_ = result

	return nil
}

// Функция выполняет сохранение в БД метрик типа gauge. Возвращает ошибку.
//
// Параметры:
//
// db - указатель на БД.
// name - имя метрики.
// value - значение метрики.
func storageDBGaugeMetrics(db *sql.DB, name string, value float64) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("ошибка сохранения метрики типа gauge в БД. В аргументе db нет указателя на БД")
	}
	if name == "" {
		return errors.New("ошибка сохранения метрики типа gauge в БД. Принято пустое значение name аргумента")
	}

	// Сохранение (обновление) метрики
	str := `
		INSERT INTO gauges (name_m, value_m) 
		VALUES ($1, $2) 
		ON CONFLICT (name_m) DO UPDATE 
		SET value_m = EXCLUDED.value_m, 
    	created_at = CURRENT_TIMESTAMP;
		`

	result, err := db.Exec(str, name, value)
	if err != nil {
		return fmt.Errorf("ошибка сохранения метрики типа gauge в БД. Не удалось сохранить метрику:<%s> с его значением:<%f>", name, value)
	}
	_ = result

	return nil
}

// Функция с комплексом действий по записи в БД принятого batch метрик, с использованием транзакции. Возвращает ошибку.
//
// Параметры:
//
// db - указатель на БД.
// m - принятый в запросе batch метрик.
func allActionsStorageBatchDBMetricsTx(db *sql.DB, m []rxMetricsBatchT) error {

	// Проверка аргументов
	if db == nil {
		return errors.New("нет указателя на БД")
	}
	if m == nil {
		return errors.New("нет массива длинных ссылок")
	}
	if len(m) == 0 {
		return errors.New("в принятом массиве длинных ссылок нет данных")
	}

	// Начало транзакции
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: <%w>", err)
	}
	defer func() {
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				log.Fatalf("аварийное прерывание работы приложения: ошибка при откате изменений в БД (метрики) <%v>", err)
			}
		}
	}()

	// Передача в БД
	for _, v := range m {
		err := storageDBMetricTx(tx, v.TypeM, v.NameM, v.ValueM)
		if err != nil {
			return fmt.Errorf("функция storageDBMetricTx вернула ошибку: ошибка <%v> сохранения метрики: тип<%s> имя<%s> значение<%s>", err, v.TypeM, v.NameM, v.ValueM)
		}
	}

	// Подтверждение транзакции
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("ошибка подтверждения транзакции: <%w>", err)
	}

	return nil
}

// Функция выполняет запись в БД с использованием транзакции. Возвращает ошибку.
//
// Параметры:
//
// tx - указатель на транзакцию.
// typeM - тип метрики.
// nameM - имя метрики.
// valueM - значение метрики.
func storageDBMetricTx(tx *sql.Tx, typeM, nameM, valueM string) error {

	// Проверка аргументов
	if tx == nil {
		return errors.New("ошибка сохранения метрики в БД. Нет указателя в аргументе tx")
	}
	if typeM == "" {
		return errors.New("ошибка сохранения метрики в БД. Принято пустое значение typeM аргумента")
	}
	if nameM == "" {
		return errors.New("ошибка сохранения метрики в БД. Принято пустое значение nameM аргумента")
	}
	if valueM == "" {
		return errors.New("ошибка сохранения метрики в БД. Принято пустое значение valueM аргумента")
	}

	// Работа с БД
	var str string
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	switch typeM {
	case "counter":

		txValueM, err := strconv.ParseInt(valueM, 10, 64)
		if err != nil {
			return fmt.Errorf("ошибка сохранения метрики в БД. Ошибка парсинга принятого значения <%s> в типе counter", valueM)
		}

		str = `
			INSERT INTO counters (name_m, value_m)
			VALUES ($1, $2)
			ON CONFLICT (name_m) 
			DO UPDATE SET value_m = EXCLUDED.value_m, created_at = CURRENT_TIMESTAMP;
		`
		_, err = tx.ExecContext(ctx, str, nameM, txValueM)
		if err != nil {
			return fmt.Errorf("ошибка сохранения метрики в БД. ошибка добавления метрики в таблицу counter <%w>", err)
		}

	case "gauge":

		txValueM, err := strconv.ParseFloat(valueM, 64)
		if err != nil {
			return fmt.Errorf("ошибка сохранения метрики в БД. Ошибка парсинга принятого значения <%s> в типе gauge", valueM)
		}

		str = `
			INSERT INTO gauges (name_m, value_m)
			VALUES ($1, $2)
			ON CONFLICT (name_m) 
			DO UPDATE SET value_m = EXCLUDED.value_m, created_at = CURRENT_TIMESTAMP;
		`
		_, err = tx.ExecContext(ctx, str, nameM, txValueM)
		if err != nil {
			return fmt.Errorf("ошибка сохранения метрики в БД. ошибка добавления метрики в таблицу gauge <%w>", err)
		}

	default:
		return fmt.Errorf("ошибка сохранения метрики в БД. Принят неподдерживаемый тип метрики <%s>", typeM)
	}

	return nil
}

// Функция выполняет сохранении принятого batch в мапы. Возвращает ошибку.
//
// Параметры:
//
// m - принятый batch.
// gM - мапа с метриками типа gauge.
// cM - мапа с метриками типа counter.
func storageMetricsInMap(m []rxMetricsBatchT, gM map[string]float64, cM map[string]int64) error {

	// Проверка аргументов
	if m == nil {
		return errors.New("в аргументе m нет указателя")
	}
	if gM == nil {
		return errors.New("в аргументе gM нет указателя")
	}
	if cM == nil {
		return errors.New("в аргументе cM нет указателя")
	}

	// Заполнение мап
	for _, v := range m {

		switch v.TypeM {
		case "counter":
			value, err := strconv.ParseInt(v.ValueM, 10, 64)
			if err != nil {
				return fmt.Errorf("ошибка парсинга значения метрики: тип<%s> имя<%s> значение<%s>", v.TypeM, v.NameM, v.ValueM)
			}
			cM[v.NameM] += value

		case "gauge":
			value, err := strconv.ParseFloat(v.ValueM, 64)
			if err != nil {
				return fmt.Errorf("ошибка парсинга значения метрики: тип<%s> имя<%s> значение<%s>", v.TypeM, v.NameM, v.ValueM)
			}
			gM[v.NameM] = value

		default:
			return fmt.Errorf("принят неподдерживаемый тип метрики: тип<%s> имя<%s> значение<%s>", v.TypeM, v.NameM, v.ValueM)
		}
	}

	return nil
}

// Функция содержит реализацию логики updateMetricByTypeAndName.
//
// Параметры:
//
// db - указатель на БД.
// m - конфигурация метрик.
// w - ResponseWriter.
// r - Request.
func internalUpdateMetricByTypeAndName(db *sql.DB, m *MetricsHandlerT, w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxData := r.URL.Path[1:]
	slRxData := strings.Split(rxData, "/")
	if len(slRxData) != 4 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	typeMetric := slRxData[1] // gauge, counter
	nameMetric := slRxData[2]
	valueMetric := slRxData[3]

	if len(nameMetric) == 0 {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Обработка сохранения
	if db != nil { // сохранение в БД

		// Сохранение
		switch typeMetric {
		case "counter":
			value, ok := m.Metrics.CounterMetrics[nameMetric]
			if !ok {
				logger.Log.Error("Ошибка, нет признака существования ключа",
					zap.String("key", nameMetric),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			err := storageDBCounterMetrics(db, nameMetric, value)
			if err != nil {
				logger.Log.Error("Ошибка при сохранении метрики в БД",
					zap.Error(err),
					zap.String("metric", nameMetric),
					zap.String("value", fmt.Sprintf("%d", value)),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

		case "gauge":
			value, ok := m.Metrics.GaugeMetrics[nameMetric]
			if !ok {
				logger.Log.Error("Ошибка, нет признака существования ключа",
					zap.String("key", nameMetric),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			err := storageDBGaugeMetrics(db, nameMetric, value)
			if err != nil {
				logger.Log.Error("Ошибка при сохранении метрики в БД",
					zap.Error(err),
					zap.String("metric", nameMetric),
					zap.String("value", fmt.Sprintf("%f", value)),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return

		}
	}

	if db == nil { // Синхронное сохранение в файл и сохранение в мапы

		// Сохранение в мапы
		switch typeMetric {
		case "counter":
			v, err := strconv.ParseInt(valueMetric, 10, 64)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			m.Metrics.CounterMetrics[nameMetric] += v

		case "gauge":
			v, err := strconv.ParseFloat(valueMetric, 64)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			m.Metrics.GaugeMetrics[nameMetric] = v

		default:
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		// Синхронное сохранение в файл
		if m.StoreIntervalMetr == "0" {
			err := storage(m.FileStoragePathMetr, m.Metrics.GaugeMetrics, m.Metrics.CounterMetrics)
			if err != nil {
				logger.Log.Error("Ошибка при синхронном сохранении в файл",
					zap.Error(err),
					zap.String("method", r.Method),
					zap.String("url", r.URL.String()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// Функция содержит реализацию логики valueMetricByTypeAndName.
//
// Параметры:
//
// m - конфигурация метрик.
// w - ResponseWriter.
// r - Request.
func internalValueMetricByTypeAndName(m *MetricsHandlerT, w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/plain")

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxData := r.URL.Path[1:]
	slRxData := strings.Split(rxData, "/")
	metricType := slRxData[1]
	metricName := slRxData[2]

	val := ""

	switch metricType {
	case "counter":
		v, ok := m.Metrics.CounterMetrics[metricName]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		val = fmt.Sprintf("%d", v)

	case "gauge":
		v, ok := m.Metrics.GaugeMetrics[metricName]
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		val = fmt.Sprintf("%f", v)

	default:
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return

	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(val))
}
