package handler

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Part001-R/YaPrShortener/internal/profile"
	"github.com/Part001-R/YaPrShortener/internal/service/flags"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ShortURLFromLong.
func Test_ShortURLFromLong_SUCCESS(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// Подготовка.
	longURL := "https://practicum.yandex.ru/"
	bodyReq := bytes.NewBuffer([]byte(longURL))

	req := httptest.NewRequest(http.MethodPost, `http://localhost:8080/`, bodyReq)
	res := httptest.NewRecorder()

	req.Header.Set("Content-Type", `text/plain`)

	// Вызов метода.
	act.ShortURLFromLong(res, req)

	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusCreated, resp.StatusCode)
}

func Test_internalShortURLFromLong_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		Observer:         nil,
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		longURLT       string
		useDBT         bool
		initMockT      func(mock sqlmock.Sqlmock)
		contentTypeT   string
		uuid           string
		wantStatusCode int
	}{
		{
			nameT:      "Запись в БД (Authorization)",
			urlT:       "http://localhost:8080",
			methodReqT: http.MethodPost,
			longURLT:   "https://practicum.yandex.ru/",
			useDBT:     true,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			contentTypeT:   "application/json",
			uuid:           "AAA",
			wantStatusCode: http.StatusCreated,
		},
		{
			nameT:      "Запись в мапы и файл",
			urlT:       "http://localhost:8080",
			methodReqT: http.MethodPost,
			longURLT:   "https://practicum.yandex.ru/",
			useDBT:     false,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			contentTypeT:   "application/json",
			uuid:           "AAA",
			wantStatusCode: http.StatusCreated,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			bodyReq := bytes.NewBuffer([]byte(tt.longURLT))

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, bodyReq)
			res := httptest.NewRecorder()

			if !tt.useDBT {
				db = nil
			}

			req.Header.Set("Content-Type", tt.contentTypeT)

			internalShortURLFromLong(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)

			if !tt.useDBT { // Проверка работы с мапами и файлом
				// Мапы
				shortURL, ok := conf.List.ShorByLong[tt.longURLT]
				assert.Equalf(t, true, ok, "нет признака существования ключа <%s> в мапе sByL", tt.longURLT)

				_, ok = conf.List.LongByShort[shortURL]
				assert.Equalf(t, true, ok, "нет признака существования ключа <%s> в мапе lByS", shortURL)

				// Файл
				copyLByS := make(map[string]string)
				for k, v := range conf.List.LongByShort {
					copyLByS[k] = v
				}
				copySByL := make(map[string]string)
				for k, v := range conf.List.ShorByLong {
					copySByL[k] = v
				}

				err = conf.LoadFileURL()
				require.NoErrorf(t, err, "неожиданная ошибка при чтении файла: <%v>", err)

				shortFromCopySByL, ok := conf.List.ShorByLong[tt.longURLT]
				require.Equalf(t, true, ok, "в локальной копии мапы sByL, нет ключа <%s>", tt.longURLT)
				assert.Equalf(t, shortURL, shortFromCopySByL, "Проверка сокращений. Нужно <%s> а принято <%s>", shortURL, shortFromCopySByL)

				longFromCopyLByS, ok := conf.List.LongByShort[shortURL]
				require.Equalf(t, true, ok, "в локальной копии мапы lByS, нет ключа <%s>", shortURL)
				assert.Equalf(t, tt.longURLT, longFromCopyLByS, "Проверка полного адреса. Нужно <%s> а принято <%s>", tt.longURLT, longFromCopyLByS)

				err = os.Remove(conf.FileStoragePath)
				assert.NoErrorf(t, err, "неожиданная ошибка при удалении файла: <%v>", err)

			} else { // Проверка работы с БД

				err = mock.ExpectationsWereMet()
				require.NoError(t, err, "не все ожидания были выполнены")
			}
		})
	}
}

func Test_internalShortURLFromLong_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		bodyT          string
		initMockT      func(mock sqlmock.Sqlmock)
		useConfT       bool
		contentTypeT   string
		wantStatusCode int
	}{
		{
			nameT:      "пустое тело",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			bodyT:      "",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       true,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:      "Нет указателя на конфигурацию",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			bodyT:      "https://practicum.yandex.ru/",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       false,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			bodyReq := bytes.NewBuffer([]byte(tt.bodyT))

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, bodyReq)
			res := httptest.NewRecorder()

			if !tt.useConfT {
				conf = nil
			}

			internalShortURLFromLong(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			assert.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидалcя код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

// ShortURLFromLongJSON.
func Test_ShortURLFromLongJSON_SUCCESS(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// Подготовка.
	testData := rxLongURL{URL: "https://practicum.yandex.ru"}

	txData, err := json.Marshal(testData)
	require.NoErrorf(t, err, "ожидалось отсутствие ошибка при маршалинге, а принято <%v>", err)

	bodyReq := bytes.NewBuffer([]byte(txData))

	req := httptest.NewRequest(http.MethodPost, `http://localhost:8080/api/shorten`, bodyReq)
	res := httptest.NewRecorder()

	req.Header.Set("Content-Type", `application/json`)

	// Вызов метода.
	act.ShortURLFromLongJSON(res, req)

	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusCreated, resp.StatusCode)
}

func Test_internalShortURLFromLongJSON_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		contentTypeT   string
		longURLT       rxLongURL
		useDBT         bool
		initMockT      func(mock sqlmock.Sqlmock)
		wantStatusCode int
	}{

		{
			nameT:        "Сохранение в БД",
			urlT:         "http://localhost:8080/api/shorten",
			methodReqT:   http.MethodPost,
			contentTypeT: `application/json`,
			longURLT:     rxLongURL{URL: "https://practicum.yandex.ru"},
			useDBT:       true,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantStatusCode: http.StatusCreated,
		},
		{
			nameT:        "Сохранение в мапы и в файл",
			urlT:         "http://localhost:8080/api/shorten",
			methodReqT:   http.MethodPost,
			contentTypeT: `application/json`,
			longURLT:     rxLongURL{URL: "https://practicum.yandex.ru"},
			useDBT:       false,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantStatusCode: http.StatusCreated,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			txData, err := json.Marshal(tt.longURLT)
			require.NoErrorf(t, err, "ожидалось отсутствие ошибка при маршалинге, а принято <%v>", err)

			bodyReq := bytes.NewBuffer([]byte(txData))

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, bodyReq)
			res := httptest.NewRecorder()

			req.Header.Set("Content-Type", tt.contentTypeT)

			if !tt.useDBT {
				db = nil
			}

			internalShortURLFromLongJSON(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код <%d>, а принят <%d>", tt.wantStatusCode, resp.StatusCode)

			if !tt.useDBT { // Проверка работы с мапами и файлом
				// Мапы
				shortURL, ok := conf.List.ShorByLong[tt.longURLT.URL]
				assert.Equalf(t, true, ok, "нет признака существования ключа <%s> в мапе sByL", tt.longURLT)

				_, ok = conf.List.LongByShort[shortURL]
				assert.Equalf(t, true, ok, "нет признака существования ключа <%s> в мапе lByS", shortURL)

				// Файл
				copyLByS := make(map[string]string)
				for k, v := range conf.List.LongByShort {
					copyLByS[k] = v
				}
				copySByL := make(map[string]string)
				for k, v := range conf.List.ShorByLong {
					copySByL[k] = v
				}

				err = conf.LoadFileURL()
				require.NoErrorf(t, err, "неожиданная ошибка при чтении файла: <%v>", err)

				shortFromCopySByL, ok := conf.List.ShorByLong[tt.longURLT.URL]
				require.Equalf(t, true, ok, "в локальной копии мапы sByL, нет ключа <%s>", tt.longURLT.URL)
				assert.Equalf(t, shortURL, shortFromCopySByL, "Проверка сокращений. Нужно <%s> а принято <%s>", shortURL, shortFromCopySByL)

				longFromCopyLByS, ok := conf.List.LongByShort[shortURL]
				require.Equalf(t, true, ok, "в локальной копии мапы lByS, нет ключа <%s>", shortURL)
				assert.Equalf(t, tt.longURLT.URL, longFromCopyLByS, "Проверка полного адреса. Нужно <%s> а принято <%s>", tt.longURLT.URL, longFromCopyLByS)

				err = os.Remove(conf.FileStoragePath)
				assert.NoErrorf(t, err, "неожиданная ошибка при удалении файла: <%v>", err)

			} else { // Проверка работы с БД

				err = mock.ExpectationsWereMet()
				require.NoError(t, err, "не все ожидания были выполнены")
			}

		})
	}
}

func Test_internalShortURLFromLongJSON_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		longURLT       rxLongURL
		initMockT      func(mock sqlmock.Sqlmock)
		useConfT       bool
		contentTypeT   string
		wantStatusCode int
	}{
		{
			nameT:      "пустое тело",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			longURLT:   rxLongURL{},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       true,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:      "Неподдерживаемый тип контента",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			longURLT:   rxLongURL{URL: "https://practicum.yandex.ru"},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       true,
			contentTypeT:   "AAA",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:      "Нет указателя на конфигурацию",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			longURLT:   rxLongURL{URL: "https://practicum.yandex.ru"},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       false,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			txData, err := json.Marshal(tt.longURLT)
			require.NoErrorf(t, err, "ожидалось отсутствие ошибка при маршалинге, а принято <%v>", err)

			bodyReq := bytes.NewBuffer([]byte(txData))

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, bodyReq)
			res := httptest.NewRecorder()

			req.Header.Set("Content-Type", tt.contentTypeT)

			if !tt.useConfT {
				conf = nil
			}

			internalShortURLFromLongJSON(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код <%d>, а принят <%d>", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

// ReadShortByLongDB.
func Test_ReadShortByLongDB(t *testing.T) {

	// Мок базы данных
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "Ошибка при создании мока")
	defer db.Close()

	// Данные для теста
	testData := []struct {
		nameTest    string
		longURL     string
		mockQuery   func()
		expectedURL string
		expectedErr error
	}{
		{
			nameTest: "URL найден",
			longURL:  "https://example.com/long-url",
			mockQuery: func() {
				mock.ExpectQuery(`SELECT short FROM shortener WHERE long = \$1`).
					WithArgs("https://example.com/long-url").
					WillReturnRows(sqlmock.NewRows([]string{"short"}).AddRow("short-url"))
			},
			expectedURL: "short-url",
			expectedErr: nil,
		},
		{
			nameTest: "URL не найден",
			longURL:  "https://example.com/unknown-url",
			mockQuery: func() {
				mock.ExpectQuery(`SELECT short FROM shortener WHERE long = \$1`).
					WithArgs("https://example.com/unknown-url").
					WillReturnError(sql.ErrNoRows)
			},
			expectedURL: "",
			expectedErr: fmt.Errorf("URL не найден: %s", "https://example.com/unknown-url"),
		},
		{
			nameTest: "Ошибка запроса",
			longURL:  "https://example.com/error-url",
			mockQuery: func() {
				mock.ExpectQuery(`SELECT short FROM shortener WHERE long = \$1`).
					WithArgs("https://example.com/error-url").
					WillReturnError(errors.New("ошибка базы данных"))
			},
			expectedURL: "",
			expectedErr: fmt.Errorf("ошибка при выполнении запроса: %v", errors.New("ошибка базы данных")),
		},
		{
			nameTest:    "Пустой longURL",
			longURL:     "",
			mockQuery:   func() {},
			expectedURL: "",
			expectedErr: errors.New("в аргументе longURL нет содержимого"),
		},
		{
			nameTest:    "nil db",
			longURL:     "https://example.com/any-url",
			mockQuery:   func() {},
			expectedURL: "",
			expectedErr: errors.New("нет указателя в аргументе db"),
		},
	}

	// Тесты.
	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {
			if tt.nameTest == "nil db" {
				_, err := readShortByLongDB(nil, tt.longURL)
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			}

			// Запуск настроек мока
			tt.mockQuery()

			// Вызов функции
			shortURL, err := readShortByLongDB(db, tt.longURL)

			// Проверка результатов
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, shortURL)
			}

			// Проверка ожиданий мока
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// LongURLFromShort.
func Test_LongURLFromShort_FAULT(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// Подготовка.
	req := httptest.NewRequest(http.MethodGet, `http://localhost:8080/EwHXdJfB`, nil)
	res := httptest.NewRecorder()

	req.Header.Set("Content-Type", `text/plain`)

	// Вызов метода.
	act.LongURLFromShort(res, req)

	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusBadRequest, resp.StatusCode)

	/*
		При выполнении теста, в лог заносится сообщение: "в мапе LongByShort, нет признака существования ключа:<EwHXdJfB>
	*/
}

func Test_LongURLFromShort_SUCCESS(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// ----- Формирование короткого представления из длинного.
	//
	// Подготовка.
	longURL := "https://practicum.yandex.ru/"
	bodyReq := bytes.NewBuffer([]byte(longURL))

	req := httptest.NewRequest(http.MethodPost, `http://localhost:8080/`, bodyReq)
	res := httptest.NewRecorder()

	req.Header.Set("Content-Type", `text/plain`)

	// Вызов метода.
	act.ShortURLFromLong(res, req)

	resp1 := res.Result()
	defer func() {
		err := resp1.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusCreated, resp1.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusCreated, resp1.StatusCode)

	// Обработка тела ответа.
	rxData, err := io.ReadAll(resp1.Body)
	rxStr := string(rxData)
	require.NoErrorf(t, err, "ошибка при чтении тела ответа, при формировании короткой ссылки:<%v>", err)

	// ----- Прлучение длинного представления по короткому.
	//
	// Подготовка.
	parsedURL, err := url.Parse(rxStr)
	if err != nil {
		fmt.Println("Ошибка парсинга URL:", err)
		return
	}

	// Формирование пути.
	path := parsedURL.Path
	path = `http://localhost:8080` + path

	req = httptest.NewRequest(http.MethodGet, path, nil)
	res = httptest.NewRecorder()

	req.Header.Set("Content-Type", `text/plain`)

	// Вызов метода.
	act.LongURLFromShort(res, req)

	resp2 := res.Result()
	defer func() {
		err := resp2.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusTemporaryRedirect, resp2.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusTemporaryRedirect, resp2.StatusCode)

	// Проверка заголовка.
	location := resp2.Header.Get("Location")

	assert.Equalf(t, longURL, location, "ожидалось содержимое заголовка:<%s>, а принято:<%s>", longURL, location)
}

func Test_internalLongURLFromShort_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Подготовка памы
	uLong := "https://practicum.yandex.ru/"
	code, err := generateCode(6)
	require.NoErrorf(t, err, "ожидалось отсутствие ошибки, а принято {%v}", err)

	conf.List.LongByShort[code] = uLong
	conf.List.ShorByLong[uLong] = code

	// Данные для теста
	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		longURLT       string
		useDBT         bool
		initMockT      func(mock sqlmock.Sqlmock)
		wantStatusCode int
	}{
		{
			nameT:      "БД. Без взведённого флага delete",
			urlT:       conf.BaseAddrShortURL + code,
			methodReqT: http.MethodGet,
			longURLT:   "https://practicum.yandex.ru/",
			useDBT:     true,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT long, deleteflag FROM shortener WHERE short = \$1`).
					WithArgs(code).
					WillReturnRows(sqlmock.NewRows([]string{"long", "deleteflag"}).AddRow("https://practicum.yandex.ru/", false))
			},
			wantStatusCode: http.StatusTemporaryRedirect,
		},
		{
			nameT:      "БД. С взведённым флагом delete",
			urlT:       conf.BaseAddrShortURL + code,
			methodReqT: http.MethodGet,
			longURLT:   "https://practicum.yandex.ru/",
			useDBT:     true,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT long, deleteflag FROM shortener WHERE short = \$1`).
					WithArgs(code).
					WillReturnRows(sqlmock.NewRows([]string{"long", "deleteflag"}).AddRow("https://practicum.yandex.ru/", true))
			},
			wantStatusCode: http.StatusGone,
		},
		{
			nameT:      "Мапы и файл",
			urlT:       conf.BaseAddrShortURL + code,
			methodReqT: http.MethodGet,
			longURLT:   "https://practicum.yandex.ru/",
			useDBT:     false,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("SELECT long").
					WithArgs(code).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantStatusCode: http.StatusTemporaryRedirect,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, nil)
			res := httptest.NewRecorder()

			if !tt.useDBT {
				db = nil
			}

			internalLongURLFromShort(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			rxStr := resp.Header.Get("Location")

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)

			if !tt.useDBT { // Проверка работы с мапами и файлом

				assert.Equalf(t, tt.longURLT, rxStr, "ожидался базовый URL <%s>, а принято <%s>", tt.longURLT, rxStr)

			} else { // Проверка работы с БД

				err = mock.ExpectationsWereMet()
				require.NoError(t, err, "не все ожидания были выполнены")
			}
		})
	}
}

func Test_internalLongURLFromShort_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Данные для теста
	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		longURLT       string
		initMockT      func(mock sqlmock.Sqlmock)
		useConf        bool
		wantStatusCode int
	}{
		{
			nameT:      "Метод POST",
			urlT:       conf.BaseAddrShortURL,
			methodReqT: http.MethodPost,
			longURLT:   "https://practicum.yandex.ru/",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT long FROM shortener`).
					WithArgs().
					WillReturnRows(sqlmock.NewRows([]string{"long"}).AddRow("https://practicum.yandex.ru/"))
			},
			useConf:        true,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:      "Нет указателя на conf",
			urlT:       conf.BaseAddrShortURL,
			methodReqT: http.MethodGet,
			longURLT:   "https://practicum.yandex.ru/",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT long FROM shortener`).
					WithArgs().
					WillReturnRows(sqlmock.NewRows([]string{"long"}).AddRow("https://practicum.yandex.ru/"))
			},
			useConf:        false,
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, nil)
			res := httptest.NewRecorder()

			if !tt.useConf {
				conf = nil
			}

			internalLongURLFromShort(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			assert.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d} а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

// internalUserURLs.
func Test_UserURLs_SUCCESS(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// ----- Формирование короткого представления из длинного.
	//
	// Подготовка.
	longURL := "https://practicum.yandex.ru/"
	bodyReq := bytes.NewBuffer([]byte(longURL))

	req1 := httptest.NewRequest(http.MethodPost, `http://localhost:8080/`, bodyReq)
	res1 := httptest.NewRecorder()

	req1.Header.Set("Content-Type", `text/plain`)

	// Вызов метода.
	act.ShortURLFromLong(res1, req1)

	resp1 := res1.Result()
	defer func() {
		err := resp1.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusCreated, resp1.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusCreated, resp1.StatusCode)

	// ----- Получение пар. GET "/api/user/urls"
	//
	req2 := httptest.NewRequest(http.MethodGet, `http://localhost:8080/api/user/urls`, nil)
	res2 := httptest.NewRecorder()

	act.UserURLs(res2, req2)

	resp2 := res2.Result()
	defer func() {
		err := resp2.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "ожидался код <%d>, а принят <%d>", http.StatusOK, resp2.StatusCode)

	// Проверка заголовка.
	resp3CT := resp2.Header.Get("Content-Type")
	require.Equalf(t, "application/json", resp3CT, "ожидался Content-Type:<%s>, а принято:<%s>", "application/json", resp3CT)

	// Обработка данных ответа.
	rxData3, err := io.ReadAll(resp2.Body)
	require.NoErrorf(t, err, "неожиданная ошибка при чтении тела ответа (3):<%v>", err)
	require.NotEqual(t, 0, len(rxData3), "нет данных ответа (3)")

	var shortLong []txShortURLOriginalURL
	err = json.Unmarshal(rxData3, &shortLong)
	require.NoErrorf(t, err, "неожиданная ошибка при десериализации ответа (3):<%v>", err)

	// Проверка присутствия длинных URL в ответе.
	longURL = strings.TrimSuffix(longURL, "/")
	shortLong[0].OriginalURL = strings.TrimSuffix(shortLong[0].OriginalURL, "/")
	require.Equalf(t, longURL, shortLong[0].OriginalURL, "нет соответствия <%s> и <%s>, первой пары", longURL, shortLong[0].OriginalURL)
}

func Test_internalUserURLs_SUCCESS(t *testing.T) {

	// Логгер.
	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Конфигурация.
	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Подготовка памы.
	uLong := "https://practicum.yandex.ru/"
	code, err := generateCode(6)
	require.NoErrorf(t, err, "ожидалось отсутствие ошибки, а принято {%v}", err)

	conf.List.LongByShort[code] = uLong

	// Данные для теста
	testData := []struct {
		nameTest       string
		urlT           string
		methodReqT     string
		wantStatusCode int
	}{

		{
			nameTest:       "Получение данных",
			urlT:           "http://localhost:123/Bar",
			methodReqT:     http.MethodGet,
			wantStatusCode: http.StatusOK,
		},
	}

	// Тесты.
	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, nil)
			res := httptest.NewRecorder()

			internalUserURLs(nil, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			// Чтение тела ответа
			body, err := io.ReadAll(resp.Body)
			require.NoErrorf(t, err, "ошибка чтения тела ответа {%v}", err)

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код:<%d>, а принят:<%d>", tt.wantStatusCode, resp.StatusCode)

			// Декодируем JSON, если это необходимо
			var rxData []txShortURLOriginalURL

			err = json.Unmarshal(body, &rxData)
			assert.NoErrorf(t, err, "ошибка декодирования JSON ответа {%v}", err)

			// Проверьте полученное тело ответа на соответствие ожидаемому

			short := conf.BaseAddrShortURL + rxData[0].ShortURL
			assert.Equalf(t, short, short, "ожидалось:<%s>, а принято:<%s>", short, rxData[0].ShortURL)
			assert.Equalf(t, uLong, rxData[0].OriginalURL, "ожидалось:<%s>, а принято:<%s>", uLong, rxData[0].OriginalURL)
		})
	}
}

// GetAllShortenerDB.
func Test_GetAllShortenerDB(t *testing.T) {
	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Мок базы данных
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "Ошибка при создании мока")
	defer db.Close()

	// Данные для теста
	testData := []struct {
		nameTest    string
		mockQuery   func()
		expectedMap map[string]string
		expectedErr error
	}{
		{
			nameTest: "Успешный запрос",
			mockQuery: func() {
				mock.ExpectQuery(`SELECT short, long FROM shortener`).
					WillReturnRows(sqlmock.NewRows([]string{"short", "long"}).
						AddRow("short-url-1", "https://example.com/long-url-1").
						AddRow("short-url-2", "https://example.com/long-url-2"))
			},
			expectedMap: map[string]string{
				"short-url-1": "https://example.com/long-url-1",
				"short-url-2": "https://example.com/long-url-2",
			},
			expectedErr: nil,
		},
		{
			nameTest: "Ошибка выполнения запроса",
			mockQuery: func() {
				mock.ExpectQuery(`SELECT short, long FROM shortener`).
					WillReturnError(errors.New("ошибка базы данных"))
			},
			expectedMap: nil,
			expectedErr: fmt.Errorf("ошибка выполнения запроса: %v", errors.New("ошибка базы данных")),
		},
		{
			nameTest:    "nil db",
			mockQuery:   func() {},
			expectedMap: nil,
			expectedErr: errors.New("в аргументе db нет указателя"),
		},
		{
			nameTest:    "nil log",
			mockQuery:   func() {},
			expectedMap: nil,
			expectedErr: errors.New("в аргументе log нет указателя"),
		},
	}

	// Тесты.
	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			if tt.nameTest == "nil db" {
				_, err := GetAllShortenerDB(nil, log)
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			}
			if tt.nameTest == "nil log" {
				_, err := GetAllShortenerDB(db, nil)
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
				return
			}

			// Запуск настроек мока
			tt.mockQuery()

			// Вызов функции
			data, err := GetAllShortenerDB(db, log)

			// Проверка ожиданий
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMap, data)
			}

			// Проверка ожиданий мока
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// LoadFileURL.
func Test_LoadFileURL_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	shortLong := NewShortenerMemory()
	shortLongHandler := &ShortLong{
		List:             shortLong,
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testData := struct {
		originalURL1 string
		shortURL1    string
		originalURL2 string
		shortURL2    string
	}{
		originalURL1: "https://practicum.yandex.ru/",
		shortURL1:    "CyevslRg",
		originalURL2: "https://AAA.ru/",
		shortURL2:    "mZ0K-YfJ",
	}

	file, err := os.OpenFile(shortLongHandler.FileStoragePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	require.NoErrorf(t, err, "неожиданная ошибка при открытии файла <%v>", err)
	defer func() {
		_ = file.Close()
	}()

	// Заполнение файла
	file.WriteString("[\n")
	str := fmt.Sprintf(`	{"uuid":"%d","short_url":"%s","original_url":"%s"},`, 1, testData.shortURL1, testData.originalURL1)
	file.WriteString(str)
	file.WriteString("\n")
	str = fmt.Sprintf(`		{"uuid":"%d","short_url":"%s","original_url":"%s"}`, 2, testData.shortURL2, testData.originalURL2)
	file.WriteString(str)
	file.WriteString("\n")
	file.WriteString("]\n")

	// Чтение файла и проверка результата
	err = shortLongHandler.LoadFileURL()
	require.NoErrorf(t, err, "ошибка чтения файла <%v>", err)

	v1, ok := shortLong.ShorByLong[testData.originalURL1]
	assert.Equalf(t, ok, true, "нет признака присутствия 1")

	v2, ok := shortLong.ShorByLong[testData.originalURL2]
	assert.Equalf(t, ok, true, "нет признака присутствия 2")

	assert.Equalf(t, testData.shortURL1, v1, "1: ожилось <%s>, а принято <%s>", testData.shortURL1, v1)
	assert.Equalf(t, testData.shortURL2, v2, "2: ожилось <%s>, а принято <%s>", testData.shortURL2, v2)

	// Удаление
	err = os.Remove(shortLongHandler.FileStoragePath)
	assert.NoErrorf(t, err, "неожиданная ошибка при удалении файла <%v>", err)
}

func Test_LoadFileURL_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	testData := []struct {
		nameT         string
		mapsShortLong *ShortLong
		wantError     string
	}{
		{
			nameT: "нет пути к файлу",
			mapsShortLong: &ShortLong{
				List: &ShortLongURL{
					ShorByLong:  map[string]string{},
					LongByShort: map[string]string{},
					mu:          sync.RWMutex{},
				},
				BaseAddrShortURL: ":8080/",
				ServerAddr:       ":8080",
				FileStoragePath:  "",
				Log:              log,
			},
			wantError: "принят пустой путь к файлу хранения",
		},
		{
			nameT: "нет указателя на ShortByLong",
			mapsShortLong: &ShortLong{
				List: &ShortLongURL{
					ShorByLong:  nil,
					LongByShort: map[string]string{},
					mu:          sync.RWMutex{},
				},
				BaseAddrShortURL: ":8080/",
				ServerAddr:       ":8080",
				FileStoragePath:  "test.json",
				Log:              log,
			},
			wantError: "нет указателя на ShortByLong",
		},
		{
			nameT: "нет указателя на LongByShort",
			mapsShortLong: &ShortLong{
				List: &ShortLongURL{
					ShorByLong:  map[string]string{},
					LongByShort: nil,
					mu:          sync.RWMutex{},
				},
				BaseAddrShortURL: ":8080/",
				ServerAddr:       ":8080",
				FileStoragePath:  "test.json",
				Log:              log,
			},
			wantError: "нет указателя на LongByShort",
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			err := tt.mapsShortLong.LoadFileURL()
			if err != nil {
				assert.Equalf(t, tt.wantError, err.Error(), "ожидалась ошибка <%s>, а принята <%s>", tt.wantError, err.Error())
			} else {
				assert.EqualError(t, err, tt.wantError)
			}
		})
	}
}

// storageDBURL.
func Test_storageDBURL_SUCCESS(t *testing.T) {

	testsData := []struct {
		nameTest string
		longURL  string
		shortURL string
		initMock func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "Корректные данные",
			longURL:  "https://practicum.yandex.ru/",
			shortURL: "EwHXdJfB",
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", "EwHXdJfB").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
	}

	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMock(mock)

			err = storageDBURLOnConflict(db, tt.longURL, tt.shortURL)
			require.NoError(t, err)
		})
	}
}

func Test_storageDBURL_FAULT(t *testing.T) {

	testsData := []struct {
		nameTest  string
		usePtrDB  bool
		longURL   string
		shortURL  string
		initMock  func(mock sqlmock.Sqlmock)
		wantError string
	}{
		{
			nameTest: "Нет указателя на БД",
			usePtrDB: false,
			longURL:  "https://practicum.yandex.ru/",
			shortURL: "EwHXdJfB",
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", "EwHXdJfB").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "нет указателя на БД в аргументе db",
		},
		{
			nameTest: "Нет данных longURL",
			usePtrDB: true,
			longURL:  "",
			shortURL: "EwHXdJfB",
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", "EwHXdJfB").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "принято пустое значение longURL аргумента",
		},
		{
			nameTest: "Нет данных shortURL",
			usePtrDB: true,
			longURL:  "https://practicum.yandex.ru/",
			shortURL: "",
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", "EwHXdJfB").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "принято пустое значение shortURL аргумента",
		},
	}

	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMock(mock)

			if !tt.usePtrDB {
				db = nil
			}

			err = storageDBURLOnConflict(db, tt.longURL, tt.shortURL)
			require.Equalf(t, tt.wantError, err.Error(), "ожидалась ошибка <%s>, а принято <%s>", tt.wantError, err.Error())
		})
	}
}

// storageBatchMap.
func Test_storageBatchMap_SUCCESS(t *testing.T) {

	testData := []struct {
		nameTest string
		batchT   []rxLongURLBatch
		sByLT    map[string]string
		lByST    map[string]string
	}{
		{
			nameTest: "корректные",
			batchT: []rxLongURLBatch{
				{
					CorrelationID: "AAA",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
				{
					CorrelationID: "BBB",
					OriginalURL:   "https://practicum.yandex.ruu/",
				},
			},
			sByLT: make(map[string]string, 0),
			lByST: make(map[string]string, 0),
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			err := storageBatchMap(tt.batchT, tt.sByLT, tt.lByST)
			require.NoErrorf(t, err, "неожиданная ошибка <%v>", err)

			for _, v := range tt.sByLT {

				_, ok := tt.lByST[v]
				assert.Equalf(t, ok, true, "в мапе lByST нет ключа с именем <%s>", v)
			}
		})
	}
}

func Test_storageBatchMap_FAULT(t *testing.T) {

	testData := []struct {
		nameTest   string
		batchT     []rxLongURLBatch
		sByLT      map[string]string
		lByST      map[string]string
		useBatchT  bool
		useLByS    bool
		useSByL    bool
		wantErrorT string
	}{
		{
			nameTest: "нет указателя на batch",
			batchT: []rxLongURLBatch{
				{
					CorrelationID: "AAA",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
				{
					CorrelationID: "BBB",
					OriginalURL:   "https://practicum.yandex.ruu/",
				},
			},
			sByLT:      make(map[string]string, 0),
			lByST:      make(map[string]string, 0),
			useBatchT:  false,
			useLByS:    true,
			useSByL:    true,
			wantErrorT: "нет указателя на batch",
		},
		{
			nameTest: "нет указателя на lByS",
			batchT: []rxLongURLBatch{
				{
					CorrelationID: "AAA",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
				{
					CorrelationID: "BBB",
					OriginalURL:   "https://practicum.yandex.ruu/",
				},
			},
			sByLT:      make(map[string]string, 0),
			lByST:      make(map[string]string, 0),
			useBatchT:  true,
			useLByS:    false,
			useSByL:    true,
			wantErrorT: "нет указателя на lByS",
		},
		{
			nameTest: "нет указателя на sByL",
			batchT: []rxLongURLBatch{
				{
					CorrelationID: "AAA",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
				{
					CorrelationID: "BBB",
					OriginalURL:   "https://practicum.yandex.ruu/",
				},
			},
			sByLT:      make(map[string]string, 0),
			lByST:      make(map[string]string, 0),
			useBatchT:  true,
			useLByS:    true,
			useSByL:    false,
			wantErrorT: "нет указателя на sByL",
		},
		{
			nameTest:   "пустой batch",
			batchT:     []rxLongURLBatch{},
			sByLT:      make(map[string]string, 0),
			lByST:      make(map[string]string, 0),
			useBatchT:  true,
			useLByS:    true,
			useSByL:    false,
			wantErrorT: "принят batch с пустым содержимым",
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			if !tt.useBatchT {
				tt.batchT = nil
			}
			if !tt.useLByS {
				tt.lByST = nil
			}
			if !tt.useSByL {
				tt.sByLT = nil
			}

			err := storageBatchMap(tt.batchT, tt.sByLT, tt.lByST)
			assert.Equalf(t, tt.wantErrorT, err.Error(), "ожидалась ошибка <%s>, а принята <%s>", tt.wantErrorT, err.Error())

		})
	}
}

// PingDB.
func Test_PingDB_SUCCESS(t *testing.T) {

	ptrDB, _, err := sqlmock.New()
	require.NoErrorf(t, err, "неожиданная ошибка: <%v>", err)

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	sl := &ShortLong{
		List: &ShortLongURL{},
		DB: &ShortLongDB{
			Ptr:         ptrDB,
			mu:          sync.RWMutex{},
			ChForDelete: make(chan DeleteDB),
			ChDoDelete:  make(chan struct{}),
		},
		Observer:         nil,
		BaseAddrShortURL: "",
		ServerAddr:       "",
		FileStoragePath:  "",
		Log:              log,
		wg:               sync.WaitGroup{},
		stopping:         false,
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	sl.PingDB(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// workWithRxData.
func Test_workWithRxData_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Подготовка данных для тестов
	testsData := []struct {
		nameTest  string
		longURLT  string
		useDB     bool
		uuid      string
		initMockT func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "сохранение в БД",
			longURLT: "https://practicum.yandex.ru/",
			useDB:    true,
			uuid:     "AAA",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nameTest: "сохранение в мапы и файл",
			longURLT: "https://practicum.yandex.ru/",
			useDB:    false,
			uuid:     "AAA",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
	}

	// тесты
	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			if !tt.useDB {
				db = nil
			}

			shortURL, err := workWithRxData(db, conf, tt.longURLT, tt.uuid)
			require.NoErrorf(t, err, "неожиданная ошибка <%v>", err)

			if !tt.useDB {
				// Проверка отработки с мапами
				value, ok := conf.List.LongByShort[shortURL]
				assert.Equalf(t, true, ok, "нет признака существования ключа <%s> в мапе lByS", shortURL)
				assert.Equalf(t, tt.longURLT, value, "ожидалось значение <%s>, а принято <%s>", tt.longURLT, value)

				// Проверка отработки с файлом
				copyLByS := make(map[string]string)
				for k, v := range conf.List.LongByShort {
					copyLByS[k] = v
				}
				copySByL := make(map[string]string)
				for k, v := range conf.List.ShorByLong {
					copySByL[k] = v
				}

				err = conf.LoadFileURL()
				require.NoErrorf(t, err, "неожиданная ошибка при чтении файла: <%v>", err)

				shortFromCopySByL, ok := conf.List.ShorByLong[tt.longURLT]
				require.Equalf(t, true, ok, "в локальной копии мапы sByL, нет ключа <%s>", tt.longURLT)
				assert.Equalf(t, shortURL, shortFromCopySByL, "Проверка сокращений. Нужно <%s> а принято <%s>", shortURL, shortFromCopySByL)

				longFromCopyLByS, ok := conf.List.LongByShort[shortURL]
				require.Equalf(t, true, ok, "в локальной копии мапы lByS, нет ключа <%s>", shortURL)
				assert.Equalf(t, tt.longURLT, longFromCopyLByS, "Проверка полного адреса. Нужно <%s> а принято <%s>", tt.longURLT, longFromCopyLByS)

				err = os.Remove(conf.FileStoragePath)
				assert.NoErrorf(t, err, "неожиданная ошибка при удалении файла: <%v>", err)

			} else {
				// Проверка всех ожиданий
				err = mock.ExpectationsWereMet()
				require.NoError(t, err, "не все ожидания были выполнены")
			}
		})
	}
}

func Test_workWithRxData_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testsData := []struct {
		nameTest    string
		usePtrConfT bool
		longURLT    string
		initMockT   func(mock sqlmock.Sqlmock)
		uuid        string
		wantErrorT  string
	}{
		{
			nameTest:    "нет значения длинной ссылки",
			usePtrConfT: true,
			longURLT:    "",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			uuid:       "AAA",
			wantErrorT: "в принятом аргументе rxLongURL, нет содержимого",
		},
		{
			nameTest:    "нет указателя на конфигурацию",
			usePtrConfT: false,
			longURLT:    "https://practicum.yandex.ru/",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			uuid:       "AAA",
			wantErrorT: "в принятом аргументе sl, нет указателя",
		},
	}

	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			if !tt.usePtrConfT {
				conf = nil
			}

			_, err = workWithRxData(db, conf, tt.longURLT, tt.uuid)
			assert.Equalf(t, tt.wantErrorT, err.Error(), "ожидалась ошибка <%s>, а принято <%s>", tt.wantErrorT, err.Error())
		})
	}
}

// Middleware.
func Test_Middleware_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Конфигурация
	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Обработчик для теста Middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mw := conf.Middleware(testHandler)

	testData := []struct {
		nameTest   string
		methodReqT string
		reqURLT    string
		encodingT  string
		wantCodeT  int
	}{
		{
			nameTest:   "успешный запрос с gzip",
			methodReqT: "http.MethodGet",
			reqURLT:    "/",
			encodingT:  "gzip",
			wantCodeT:  http.StatusOK,
		},
	}

	// Тесты.
	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			req := httptest.NewRequest(tt.methodReqT, tt.reqURLT, nil)
			req.Header.Set("Accept-Encoding", tt.encodingT)
			rr := httptest.NewRecorder()

			mw.ServeHTTP(rr, req)

			reader, err := gzip.NewReader(rr.Body)
			if err != nil {
				t.Fatalf("Ошибка при создании gzip reader: %s", err)
			}
			defer reader.Close()

			decompressedBody, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("Ошибка при чтении декомпрессированного тела: %s", err)
			}

			assert.Equal(t, tt.wantCodeT, rr.Code)
			assert.Equal(t, "OK", string(decompressedBody))

		})
	}
}

func Test_Middleware_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Конфигурация
	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Обработчик для теста Middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mw := conf.Middleware(testHandler)

	testData := []struct {
		nameTest         string
		methodReqT       string
		reqURLT          string
		acceptEncodingT  string
		contentEncodingT string
		wantCodeT        int
	}{
		{
			nameTest:         "неподдерживаемая запрашиваемая кодировка",
			methodReqT:       http.MethodGet,
			reqURLT:          "/",
			acceptEncodingT:  "AAA",
			contentEncodingT: "gzip",
			wantCodeT:        http.StatusBadRequest,
		},
		{
			nameTest:         "неподдерживаемая принятая кодировка",
			methodReqT:       http.MethodGet,
			reqURLT:          "/",
			acceptEncodingT:  "gzip",
			contentEncodingT: "AAA",
			wantCodeT:        http.StatusBadRequest,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			req := httptest.NewRequest(tt.methodReqT, tt.reqURLT, nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncodingT)
			req.Header.Set("Content-Encoding", tt.contentEncodingT)
			rr := httptest.NewRecorder()

			mw.ServeHTTP(rr, req)

			assert.Equalf(t, http.StatusBadRequest, rr.Code, "ожидался код <%d>, а принят <%d>", http.StatusBadRequest, rr.Code)
		})
	}
}

func Test_MiddlewareAudit_SUCCESS(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// Обработчик для теста Middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mw := act.Middleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:3489/", nil)
	res := httptest.NewRecorder()

	// Запуск функции.
	mw.ServeHTTP(res, req)
	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Обработка ответа
	rxByte, err := io.ReadAll(resp.Body)
	require.NoErrorf(t, err, "ошибка чтения тела ответа:<%v>", err)

	rxStr := string(rxByte)
	wantStr := "OK"
	assert.Equalf(t, wantStr, rxStr, "ожадалось сообщение:<%s>, а принято:<%s>", wantStr, rxStr)
}

// ShortURLFromLongBatch.
func Test_ShortURLFromLongBatch_SUCCESS(t *testing.T) {

	// Создание интерфейса.
	act, err := constructor()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// Подготовка.
	txData := []rxLongURLBatch{{
		CorrelationID: "ID001",
		OriginalURL:   "https://practicum.yandex.ru",
	}}

	txByte, err := json.Marshal(txData)
	require.NoErrorf(t, err, "ожидалось отсутствие ошибка при маршалинге, а принято <%v>", err)

	bodyReq := bytes.NewBuffer([]byte(txByte))

	req := httptest.NewRequest(http.MethodPost, `http://localhost:8080/api/shorten/batch`, bodyReq)
	res := httptest.NewRecorder()

	req.Header.Set("Content-Type", `application/json`)

	// Вызов метода.
	act.ShortURLFromLongBatch(res, req)

	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "ожидался код:<%d>, а принят:<%d>", http.StatusCreated, resp.StatusCode)

	// Проверка Content-Type.
	respCT := resp.Header.Get("Content-Type")
	require.Equalf(t, `application/json`, respCT, "ожидался Content-Type:<%s>, а принят:<%s>", `application/json`, respCT)

	// Проверка данных ответа.
	rxByte, err := io.ReadAll(resp.Body)
	require.NoErrorf(t, err, "ошибка чтения тела ответа:<%v>", err)

	var rxData []txShortURLBatch
	json.Unmarshal(rxByte, &rxData)

	assert.Equalf(t, txData[0].CorrelationID, rxData[0].CorrelationID, "ожидался ID:<%s> а принят:<%s>", txData[0].CorrelationID, rxData[0].CorrelationID)
}

func Test_internalShortURLFromLongBatch_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Конфигурация
	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Данные для тестов
	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		batchLongURLT  []rxLongURLBatch
		initMockT      func(mock sqlmock.Sqlmock)
		useDB          bool
		wantStatusCode int
	}{
		{
			nameT:      "БД",
			urlT:       "http://localhost:8080/api/shorten/batch",
			methodReqT: http.MethodPost,
			batchLongURLT: []rxLongURLBatch{
				{
					CorrelationID: "12345",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
				{
					CorrelationID: "67890",
					OriginalURL:   "https://example.com/",
				},
			},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectExec("INSERT INTO").
					WithArgs("https://example.com/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(2, 1))

				mock.ExpectCommit()
			},
			useDB:          true,
			wantStatusCode: http.StatusCreated,
		},
		{
			nameT:      "Мапы",
			urlT:       "http://localhost:8080/api/shorten/batch",
			methodReqT: http.MethodPost,
			batchLongURLT: []rxLongURLBatch{
				{
					CorrelationID: "11111",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
				{
					CorrelationID: "22222",
					OriginalURL:   "https://example.com/",
				},
			},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectExec("INSERT INTO").
					WithArgs("https://example.com/", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(2, 1))

				mock.ExpectCommit()
			},
			useDB:          false,
			wantStatusCode: http.StatusCreated,
		},
	}

	// Тесты
	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			dataByte, err := json.Marshal(tt.batchLongURLT)
			require.NoErrorf(t, err, "ошибка при сериализации данных")

			body := bytes.NewBuffer(dataByte)

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, body)
			res := httptest.NewRecorder()

			req.Header.Set("Content-Type", "application/json")

			if !tt.useDB {
				db = nil
			}

			internalShortURLFromLongBatch(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код <%d>, а принят <%d>", tt.wantStatusCode, resp.StatusCode)

			// Результат
			rxByte, err := io.ReadAll(resp.Body)
			require.NoErrorf(t, err, "ошибка при чтении тела ответа")

			batchShortURL := make([]txShortURLBatch, 0)

			err = json.Unmarshal(rxByte, &batchShortURL)
			require.NoErrorf(t, err, "ошибка десериализации")

			require.Equalf(t, tt.batchLongURLT[0].CorrelationID, batchShortURL[0].CorrelationID, "ожидалось <%s>, а принято <%s>", tt.batchLongURLT[0].CorrelationID, batchShortURL[0].CorrelationID)
			require.Equalf(t, tt.batchLongURLT[1].CorrelationID, batchShortURL[1].CorrelationID, "ожидалось <%s>, а принято <%s>", tt.batchLongURLT[1].CorrelationID, batchShortURL[1].CorrelationID)

			assert.NotEqualf(t, 0, len(batchShortURL[0].ShortURL), "нет содержимого сокращения в ответе у id <%d>", 0)
			assert.NotEqualf(t, 0, len(batchShortURL[1].ShortURL), "нет содержимого сокращения в ответе у id <%d>", 1)

			// Проверка, что все mock выполнены
			if tt.useDB {
				err = mock.ExpectationsWereMet()
				require.NoError(t, err, "не все ожидания были выполнены")
			}

			// Удаление созданного файла.
			os.Remove("storage.json")
		})
	}
}

func Test_internalShortURLFromLongBatch_FAULT(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		bodyT          string
		initMockT      func(mock sqlmock.Sqlmock)
		useConfT       bool
		contentTypeT   string
		wantStatusCode int
	}{
		{
			nameT:      "Неподдерживаемый метод",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodGet,
			bodyT:      "https://practicum.yandex.ru/",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       true,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:      "пустое тело",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			bodyT:      "",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       true,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:      "Нет указателя на конфигурацию",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			bodyT:      "https://practicum.yandex.ru/",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			useConfT:       false,
			contentTypeT:   "application/json",
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			bodyReq := bytes.NewBuffer([]byte(tt.bodyT))

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, bodyReq)
			res := httptest.NewRecorder()

			if !tt.useConfT {
				conf = nil
			}

			internalShortURLFromLongBatch(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			assert.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидалcя код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

// DeleteUserURLs.
func Test_DeleteUserURLs_FAULT(t *testing.T) {

	// Создание интерфейса.
	act, mock, err := constructorDB()
	require.NoErrorf(t, err, "неожиданная ошибка в функции constructor:<%v>", err)

	// Подготовка.
	txData := []string{"foo", "bar"}

	txByte, err := json.Marshal(txData)
	require.NoErrorf(t, err, "ожидалось отсутствие ошибка при маршалинге, а принято <%v>", err)

	bodyReq := bytes.NewBuffer([]byte(txByte))

	req := httptest.NewRequest(http.MethodDelete, `http://localhost:8080/api/user/urls`, bodyReq)
	res := httptest.NewRecorder()

	auth := "foo"
	req.Header.Set("Authorization", auth)

	// Вызов метода.
	act.DeleteUserURLs(res, req)

	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
	}()

	// Проверка статуса.
	//
	// ...

	_ = mock
}

func Test_InternalDeleteUserURLs_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Данные для тестов
	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		shortT         []string
		initMockT      func(mock sqlmock.Sqlmock)
		useDB          bool
		aythorizT      string
		wantStatusCode int
	}{
		{
			nameT:      "БД (авторизация)",
			urlT:       "http://localhost:8080/api/user/urls",
			methodReqT: http.MethodDelete,
			shortT:     []string{"short1", "short2"},
			initMockT: func(mock sqlmock.Sqlmock) {

				//mock.ExpectBegin()

				// Указываем полный запрос для удаления
				mock.ExpectExec("DELETE FROM shortener WHERE short = $1").
					WithArgs("short1").
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectExec("DELETE FROM shortener WHERE short = $1").
					WithArgs("short2").
					WillReturnResult(sqlmock.NewResult(0, 1))

				//mock.ExpectCommit()
			},
			useDB:          true,
			aythorizT:      "AAA",
			wantStatusCode: http.StatusAccepted,
		},
	}

	// Тесты
	for _, tt := range testData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			body, err := json.Marshal(tt.shortT)
			require.NoErrorf(t, err, "неожиданная ошибка при сериализации данных: <%v>", err)

			req := httptest.NewRequest(tt.methodReqT, tt.urlT, bytes.NewBuffer(body))
			res := httptest.NewRecorder()

			req.Header.Set("Authorization", tt.aythorizT)

			if !tt.useDB {
				db = nil
			}

			internalDeleteUserURLs(db, conf, res, req)

			resp := res.Result()
			defer resp.Body.Close()

			assert.Equalf(t, http.StatusAccepted, resp.StatusCode, "ожидася код <%d>, а принят <%d>", http.StatusAccepted, resp.StatusCode)

			if tt.useDB {
				err = mock.ExpectationsWereMet()
				assert.NoErrorf(t, err, "неожиданная ошибка при проверке выполнения mock: <%v>", err)
			}

		})
	}
}

func Benchmark_InternalDeleteUserURLs_SUCCESS(b *testing.B) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(b, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Подготовка конфигурации
	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
	}

	// Данные для тестов
	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		shortT         []string
		initMockT      func(mock sqlmock.Sqlmock)
		useDB          bool
		aythorizT      string
		wantStatusCode int
	}{
		{
			nameT:      "БД (авторизация)",
			urlT:       "http://localhost:8080/api/user/urls",
			methodReqT: http.MethodDelete,
			shortT:     []string{"short1", "short2"},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM shortener WHERE short = $1").
					WithArgs("short1").
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectExec("DELETE FROM shortener WHERE short = $1").
					WithArgs("short2").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			useDB:          true,
			aythorizT:      "AAA",
			wantStatusCode: http.StatusAccepted,
		},
	}

	// Тесты
	for i := 0; i < b.N; i++ {
		for _, tt := range testData {

			b.Run(tt.nameT, func(b *testing.B) {
				db, mock, err := sqlmock.New()
				require.NoError(b, err)
				defer db.Close()

				body, err := json.Marshal(tt.shortT)
				require.NoErrorf(b, err, "неожиданная ошибка при сериализации данных: <%v>", err)

				req := httptest.NewRequest(tt.methodReqT, tt.urlT, bytes.NewBuffer(body))
				res := httptest.NewRecorder()

				req.Header.Set("Authorization", tt.aythorizT)

				if !tt.useDB {
					db = nil
				}

				b.ResetTimer()

				internalDeleteUserURLs(db, conf, res, req)

				resp := res.Result()
				defer resp.Body.Close()

				assert.Equalf(b, http.StatusAccepted, resp.StatusCode, "ожидася код <%d>, а принят <%d>", http.StatusAccepted, resp.StatusCode)

				if tt.useDB {
					err = mock.ExpectationsWereMet()
					assert.NoErrorf(b, err, "неожиданная ошибка при проверке выполнения mock: <%v>", err)
				}
			})
		}
	}
	// Запуск профилирования памяти в конце теста
	closeFileMem, err := profile.Memory(log)
	require.NoErrorf(b, err, "неожиданная ошибка запуске профилирования: <%v>", err)

	err = closeFileMem()
	require.NoErrorf(b, err, "неожиданная ошибка при закрытии файла профилирования памяти: <%v>", err)
}

// allActionsStorageBatchDBURL.
func Test_allActionsStorageBatchDBURL_SUCCESS(t *testing.T) {

	// Подготовка данных для тестов
	testsData := []struct {
		nameTest  string
		batchT    []rxLongURLBatch
		initMockT func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "сохранение в БД",
			batchT: []rxLongURLBatch{
				{
					CorrelationID: "AAA",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
			},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), "AAA").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
		},
	}

	// тесты
	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			shortData, err := allActionsStorageBatchDBURL(db, tt.batchT, "http://localhost:8080/", "AAA")
			require.NoErrorf(t, err, "ошибка при работе с БД <%v>", err)

			assert.Equalf(t, len(tt.batchT), len(shortData), "ожидаемая длинна слайса <%d> не соответствует полученному <%d>", len(tt.batchT), len(shortData))

			// Проверка всех ожиданий
			err = mock.ExpectationsWereMet()
			require.NoError(t, err, "не все ожидания были выполнены")
		})
	}
}

func Test_allActionsStorageBatchDBURL_FAULT(t *testing.T) {

	// Подготовка данных для тестов
	testsData := []struct {
		nameTest   string
		batchT     []rxLongURLBatch
		initMockT  func(mock sqlmock.Sqlmock)
		useDBT     bool
		useBatchT  bool
		wantErrorT string
	}{
		{
			nameTest: "нет указателя на БД",
			batchT: []rxLongURLBatch{
				{
					CorrelationID: "AAA",
					OriginalURL:   "https://practicum.yandex.ru/",
				},
			},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			useDBT:     false,
			useBatchT:  true,
			wantErrorT: "нет указателя на БД",
		},
		{
			nameTest: "пустой batch",
			batchT:   []rxLongURLBatch{},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			useDBT:     true,
			useBatchT:  true,
			wantErrorT: "в принятом массиве длинных ссылок нет данных",
		},
		{
			nameTest: "нет указателя на batch",
			batchT:   []rxLongURLBatch{},
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg(), "AAA").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			useDBT:     true,
			useBatchT:  false,
			wantErrorT: "нет указателя на batch",
		},
	}

	// тесты
	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			if !tt.useDBT {
				db = nil
			}
			if !tt.useBatchT {
				tt.batchT = nil
			}

			rx, err := allActionsStorageBatchDBURL(db, tt.batchT, "http://localhost:8080/", "AAA")
			_ = rx

			assert.Equalf(t, tt.wantErrorT, err.Error(), "ожидалась ошибка <%s> а принято <%s>", tt.wantErrorT, err.Error())

		})
	}
}

// Stats.
//
// Проверка статусов ответа.
func Test_Stats_Status(t *testing.T) {

	// Подготовка.
	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Данные для теста.
	dataTest := []struct {
		nameTest           string
		IPTest             string
		wantStatusCode     int
		clearTrustedSubnet bool
	}{
		{
			nameTest:           "Есть вхождение",
			IPTest:             "127.0.0.10",
			wantStatusCode:     http.StatusOK,
			clearTrustedSubnet: false,
		},
		{
			nameTest:           "Нет вхождения",
			IPTest:             "192.0.0.10",
			wantStatusCode:     http.StatusForbidden,
			clearTrustedSubnet: false,
		},
		{
			nameTest:           "Нет значения в заголовке",
			IPTest:             "",
			wantStatusCode:     http.StatusForbidden,
			clearTrustedSubnet: false,
		},
		{
			nameTest:           "Нет инициализации подсети",
			IPTest:             "192.0.0.10",
			wantStatusCode:     http.StatusForbidden,
			clearTrustedSubnet: true,
		},
	}

	// Тесты.
	for _, tt := range dataTest {
		t.Run(tt.nameTest, func(t *testing.T) {

			// Проверка на сброс содержимого.
			if tt.clearTrustedSubnet {
				conf.TrustedSubnet = ""
			}

			// Подготовка обработчиков.
			r := chi.NewRouter()
			r.Use(conf.MiddlewareCountConnect)
			r.Use(conf.MiddlewareTrustSubnet)
			r.Use(conf.Middleware)
			r.Get("/api/internal/stats", http.HandlerFunc(conf.Stats))

			// Запрос.
			req, err := http.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if err != nil {
				require.NoErrorf(t, err, "ошибка при создании запроса: <%v>", err)
			}
			req.Header.Set("X-Real-IP", tt.IPTest)

			// Рекордер.
			res := httptest.NewRecorder()

			// Вызов обработчика.
			r.ServeHTTP(res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			// Проверка.
			assert.Equalf(t, tt.wantStatusCode, res.Code, "ожидался код ответа:<%d>, а принят:<%d>", tt.wantStatusCode, res.Code)
		})
	}
}

// Проверка получения статистики.
func Test_Stats_Request(t *testing.T) {

	// Подготовка.
	//
	// Логгер
	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Конфигурация сервиса.
	conf := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
		Log:              log,
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Наполнение in-memory парами соответствия.
	conf.List.LongByShort["long-1"] = "short-1"
	conf.List.LongByShort["long-2"] = "short-2"
	conf.List.LongByShort["long-3"] = "short-3"

	// Данные для теста.
	dataTest := []struct {
		nameTest       string
		IPTest         string
		wantStatusCode int
		wantData       txStats
	}{
		{
			nameTest:       "Взаимодействие с in-memory.",
			IPTest:         "127.0.0.10",
			wantStatusCode: http.StatusOK,
			wantData: txStats{
				URLs:  3,
				Users: 1,
			},
		},
	}

	// Тесты.
	for _, tt := range dataTest {
		t.Run(tt.nameTest, func(t *testing.T) {

			// Подготовка обработчиков.
			r := chi.NewRouter()
			r.Use(conf.MiddlewareCountConnect)
			r.Use(conf.MiddlewareTrustSubnet)
			r.Use(conf.Middleware)
			r.Get("/api/internal/stats", http.HandlerFunc(conf.Stats))

			// Запрос.
			req, err := http.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if err != nil {
				require.NoErrorf(t, err, "ошибка при создании запроса: <%v>", err)
			}
			req.Header.Set("X-Real-IP", tt.IPTest)

			// Рекордер.
			res := httptest.NewRecorder()

			// Вызов обработчика.
			r.ServeHTTP(res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка закрытия потока {%v}", err)
			}()

			// Тело ответа.
			rxByte, err := io.ReadAll(resp.Body)
			require.NoErrorf(t, err, "ошибка при чтении тела запроса:<%v>", err)

			var rxData txStats
			err = json.Unmarshal(rxByte, &rxData)
			require.NoErrorf(t, err, "ошибка десериализации:<%v>", err)

			// Проверка.
			require.Equalf(t, tt.wantStatusCode, res.Code, "ожидался код ответа:<%d>, а принят:<%d>", tt.wantStatusCode, res.Code)
			assert.Equalf(t, tt.wantData.URLs, rxData.URLs, "для URLs ожидалось:<%d>, а принято:<%d>", tt.wantData.URLs, rxData.URLs)
			assert.Equalf(t, tt.wantData.Users, rxData.Users, "для Users ожидалось:<%d>, а принято:<%d>", tt.wantData.Users, rxData.Users)
		})
	}
}

// ---

// Конструкторы.
func Test_NewShortenerMemory_SUCCESS(t *testing.T) {

	inst := NewShortenerMemory()

	assert.NotNil(t, inst.LongByShort, "отсутствует указатель на LongByShort")
	assert.NotNil(t, inst.ShorByLong, "отсутствует указатель на ShorByLong")
}

func Test_NewShortenerDB_SUCCESS(t *testing.T) {

	var db *sql.DB

	inst := NewShortenerDB(db)
	assert.NotNil(t, inst.ChDoDelete, "отсутствует указатель на ChDoDelete")
	assert.NotNil(t, inst.ChForDelete, "отсутствует указатель на ChForDelete")
	assert.Nil(t, inst.Ptr, "должен отсутствовать указатель")
}

func Test_NewShortener_SUCCESS(t *testing.T) {

	// Экземляр in memory
	storage := &ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
		mu:          sync.RWMutex{},
	}

	// БД
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	instDB := NewShortenerDB(db)

	// Флаги
	fl := &flags.Config{
		Port:             "A",
		BaseAddrShortURL: "B",
		LogLevel:         "C",
		FileStoragePath:  "D",
		AuditFile:        "E",
		AuditURL:         "F",
		DSNDB:            "G",
		EnableHTTPS:      "H",
		ConfigFile:       "I",
	}

	// Конструктор логгера.
	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Тест.
	inst := NewShortener(storage, instDB, fl, obsSrc, log, "")
	assert.NotNil(t, inst, "отсутствует указатель")
}

// ---

// ClearShortenerTable.
func Test_ClearShortenerTable(t *testing.T) {

	// mock для базы данных
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Тест успешного запроса.
	t.Run("Success", func(t *testing.T) {
		mock.ExpectExec(`TRUNCATE TABLE shortener RESTART IDENTITY;`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err = ClearShortenerTable(db)
		require.NoError(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	// Тест при nil db.
	t.Run("NilDB", func(t *testing.T) {
		err := ClearShortenerTable(nil)
		assert.EqualError(t, err, "нет указателя в аргументе db")
	})

	// Тест ошибки.
	t.Run("ExecError", func(t *testing.T) {
		mock.ExpectExec(`TRUNCATE TABLE shortener RESTART IDENTITY;`).
			WillReturnError(errors.New("ошибка выполнения"))

		err = ClearShortenerTable(db)
		assert.EqualError(t, err, "ошибка при очистке таблицы: <ошибка выполнения>")

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// WaitFinActions.
func Test_WaitFinActions_SUCCESS(t *testing.T) {
	sl := &ShortLong{}

	sl.wg.Add(1)

	// Запуск горутины
	go func() {
		time.Sleep(100 * time.Millisecond)
		sl.wg.Done()
	}()

	// Ожидание
	start := time.Now()
	sl.WaitFinActions()
	duration := time.Since(start)

	// Проверка ожидания
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "WaitFinActions завершилась слишком быстро")
}

// SetFlagStopping.
func Test_SetFlagStopping_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	sl := &ShortLong{
		List:             &ShortLongURL{},
		DB:               &ShortLongDB{},
		Observer:         nil,
		BaseAddrShortURL: "",
		ServerAddr:       "",
		FileStoragePath:  "",
		Log:              log,
		wg:               sync.WaitGroup{},
		stopping:         false,
	}

	before := sl.IsFlagStopping()
	sl.SetFlagStopping()
	after := sl.IsFlagStopping()

	assert.NotEqualf(t, before, after, "значения должны отличаться. до запуска:<%t> и после:<%t>", before, after)
}

// IsFlagStopping.
func Test_IsFlagStopping_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	sl := &ShortLong{
		List:             &ShortLongURL{},
		DB:               &ShortLongDB{},
		Observer:         nil,
		BaseAddrShortURL: "",
		ServerAddr:       "",
		FileStoragePath:  "",
		Log:              log,
		wg:               sync.WaitGroup{},
		stopping:         false,
	}

	// Тест.
	val := sl.IsFlagStopping()
	assert.Equal(t, false, val, "значение должно быть false")

	sl.SetFlagStopping()
	val = sl.IsFlagStopping()
	assert.Equal(t, true, val, "значение должно быть true")
}

// AsyncDeleteWGAdd.
func Test_AsyncDeleteWGAdd_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	sl := &ShortLong{
		List:             &ShortLongURL{},
		DB:               &ShortLongDB{},
		Observer:         nil,
		BaseAddrShortURL: "",
		ServerAddr:       "",
		FileStoragePath:  "",
		Log:              log,
		wg:               sync.WaitGroup{},
		stopping:         false,
	}

	sl.WGAdd() // wg.Add(1)

	go func() {
		defer sl.WGDone() // wg.Done()
		time.Sleep(2 * time.Second)
	}()

	time.Sleep(100 * time.Millisecond)

	tStart := time.Now()
	sl.WaitFinActions() // wg.Wait()
	duration := time.Since(tStart)

	// Проверка ожидания
	assert.GreaterOrEqual(t, duration, 1000*time.Millisecond, "WaitFinActions завершилась слишком быстро")
}

// AsyncDeleteWGDone.
func Test_AsyncDeleteWGDone_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	sl := &ShortLong{
		List:             &ShortLongURL{},
		DB:               &ShortLongDB{},
		Observer:         nil,
		BaseAddrShortURL: "",
		ServerAddr:       "",
		FileStoragePath:  "",
		Log:              log,
		wg:               sync.WaitGroup{},
		stopping:         false,
	}

	sl.WGAdd() // wg.Add(1)

	go func() {
		defer sl.WGDone() // wg.Done()
		time.Sleep(2 * time.Second)
	}()

	time.Sleep(100 * time.Millisecond)

	tStart := time.Now()
	sl.WaitFinActions() // wg.Wait()
	duration := time.Since(tStart)

	// Проверка ожидания
	assert.GreaterOrEqual(t, duration, 1000*time.Millisecond, "WaitFinActions завершилась слишком быстро")
}

// ---

// constructor, конструктор сервиса, для тестов. Возвращает интерфейс и ошибка.
func constructor() (Actions, error) {

	// Флаги
	fl := &flags.Config{
		Port:             "localhost:8080",
		BaseAddrShortURL: "http://localhost",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "",
		EnableHTTPS:      "false",
		ConfigFile:       "",
	}

	// in memory
	storage := &ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
		mu:          sync.RWMutex{},
	}

	// БД
	var dbPtr *sql.DB
	instDB := NewShortenerDB(dbPtr)

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	if err != nil {
		return nil, fmt.Errorf("Ошибка при создании логгера:<%v>", err)
	}

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Конструктор сервиса.
	return NewShortener(storage, instDB, fl, obsSrc, log, ""), nil
}

// constructorDB, конструктор сервиса, для тестов. Возвращает интерфейс, mock БД и ошибку.
func constructorDB() (Actions, sqlmock.Sqlmock, error) {

	// Флаги
	fl := &flags.Config{
		Port:             "localhost:8080",
		BaseAddrShortURL: "http://localhost",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "host=localhost port=5432 user=foo password=bar dbname=oups  sslmode=disable",
		EnableHTTPS:      "false",
		ConfigFile:       "",
	}

	// in memory
	storage := &ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
		mu:          sync.RWMutex{},
	}

	// БД
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil, fmt.Errorf("Ошибка при создании sqlmock:<%v>", err)
	}
	instDB := NewShortenerDB(db)

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	if err != nil {
		return nil, nil, fmt.Errorf("Ошибка при создании логгера:<%v>", err)
	}

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Конструктор сервиса.
	return NewShortener(storage, instDB, fl, obsSrc, log, ""), mock, nil
}
