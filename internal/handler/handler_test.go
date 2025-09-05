package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// URL

func Test_internalShortURLFromLong_SUCCESS(t *testing.T) {

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		longURLT       string
		useDBT         bool
		initMockT      func(mock sqlmock.Sqlmock)
		contentTypeT   string
		wantStatusCode int
	}{
		{
			nameT:      "Запись в БД",
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

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
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

func Test_internalShortURLFromLongJSON_SUCCESS(t *testing.T) {

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		contentTypeT   string
		longURLT       rxLongURLT
		useDBT         bool
		initMockT      func(mock sqlmock.Sqlmock)
		wantStatusCode int
	}{

		{
			nameT:        "Сохранение в БД",
			urlT:         "http://localhost:8080/api/shorten",
			methodReqT:   http.MethodPost,
			contentTypeT: `application/json`,
			longURLT:     rxLongURLT{URL: "https://practicum.yandex.ru"},
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
			longURLT:     rxLongURLT{URL: "https://practicum.yandex.ru"},
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

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		longURLT       rxLongURLT
		initMockT      func(mock sqlmock.Sqlmock)
		useConfT       bool
		contentTypeT   string
		wantStatusCode int
	}{
		{
			nameT:      "пустое тело",
			urlT:       "http://localhost:8080/",
			methodReqT: http.MethodPost,
			longURLT:   rxLongURLT{},
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
			longURLT:   rxLongURLT{URL: "https://practicum.yandex.ru"},
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
			longURLT:   rxLongURLT{URL: "https://practicum.yandex.ru"},
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

func Test_internalLongURLFromShort_SUCCESS(t *testing.T) {

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
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

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
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

func Test_LoadFileURL_SUCCESS(t *testing.T) {

	shortLong := NewShortLongURL()
	shortLongHandler := &ShortLongT{
		List:             shortLong,
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
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

	// Чтение файла и проыерка результата
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

	testData := []struct {
		nameT         string
		mapsShortLong *ShortLongT
		wantError     string
	}{
		{
			nameT: "нет пути к файлу",
			mapsShortLong: &ShortLongT{
				List: &ShortLongURLT{
					ShorByLong:  map[string]string{},
					LongByShort: map[string]string{},
					Mu:          sync.RWMutex{},
				},
				BaseAddrShortURL: ":8080/",
				ServerAddr:       ":8080",
				FileStoragePath:  "",
			},
			wantError: "принят пустой путь к файлу хранения",
		},
		{
			nameT: "нет указателя на ShortByLong",
			mapsShortLong: &ShortLongT{
				List: &ShortLongURLT{
					ShorByLong:  nil,
					LongByShort: map[string]string{},
					Mu:          sync.RWMutex{},
				},
				BaseAddrShortURL: ":8080/",
				ServerAddr:       ":8080",
				FileStoragePath:  "test.json",
			},
			wantError: "нет указателя на ShortByLong",
		},
		{
			nameT: "нет указателя на LongByShort",
			mapsShortLong: &ShortLongT{
				List: &ShortLongURLT{
					ShorByLong:  map[string]string{},
					LongByShort: nil,
					Mu:          sync.RWMutex{},
				},
				BaseAddrShortURL: ":8080/",
				ServerAddr:       ":8080",
				FileStoragePath:  "test.json",
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

func Test_ShortURLFromLongBatch_SUCCESS(t *testing.T) {

	// Подготовка данных для тестов
	testsData := []struct {
		nameTest  string
		batchT    []rxLongURLBatchT
		initMockT func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "сохранение в БД",
			batchT: []rxLongURLBatchT{
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

func Test_ShortURLFromLongBatch_FAULT(t *testing.T) {

	// Подготовка данных для тестов
	testsData := []struct {
		nameTest   string
		batchT     []rxLongURLBatchT
		initMockT  func(mock sqlmock.Sqlmock)
		useDBT     bool
		useBatchT  bool
		wantErrorT string
	}{
		{
			nameTest: "нет указателя на БД",
			batchT: []rxLongURLBatchT{
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
			batchT:   []rxLongURLBatchT{},
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
			batchT:   []rxLongURLBatchT{},
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

func Test_storageBatchMap_SUCCESS(t *testing.T) {

	testData := []struct {
		nameTest string
		batchT   []rxLongURLBatchT
		sByLT    map[string]string
		lByST    map[string]string
	}{
		{
			nameTest: "корректные",
			batchT: []rxLongURLBatchT{
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
		batchT     []rxLongURLBatchT
		sByLT      map[string]string
		lByST      map[string]string
		useBatchT  bool
		useLByS    bool
		useSByL    bool
		wantErrorT string
	}{
		{
			nameTest: "нет указателя на batch",
			batchT: []rxLongURLBatchT{
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
			batchT: []rxLongURLBatchT{
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
			batchT: []rxLongURLBatchT{
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
			batchT:     []rxLongURLBatchT{},
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

func Test_workWithRxData_SUCCESS(t *testing.T) {

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Подготовка данных для тестов
	testsData := []struct {
		nameTest  string
		longURLT  string
		useDB     bool
		initMockT func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "сохранение в БД",
			longURLT: "https://practicum.yandex.ru/",
			useDB:    true,
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

			shortURL, uuid, err := workWithRxData(db, conf, tt.longURLT)
			_ = uuid
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
	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	testsData := []struct {
		nameTest    string
		usePtrConfT bool
		longURLT    string
		initMockT   func(mock sqlmock.Sqlmock)
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

			_, _, err = workWithRxData(db, conf, tt.longURLT)
			assert.Equalf(t, tt.wantErrorT, err.Error(), "ожидалась ошибка <%s>, а принято <%s>", tt.wantErrorT, err.Error())
		})
	}
}

func Test_Middleware_SUCCESS(t *testing.T) {

	// Обработчик для теста Middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mw := Middleware(testHandler)

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

	// Обработчик для теста Middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mw := Middleware(testHandler)

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

func Test_internalShortURLFromLongBatch_SUCCESS(t *testing.T) {

	// Конфигурация
	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Данные для тестов
	testData := []struct {
		nameT          string
		urlT           string
		methodReqT     string
		batchLongURLT  []rxLongURLBatchT
		initMockT      func(mock sqlmock.Sqlmock)
		useDB          bool
		wantStatusCode int
	}{
		{
			nameT:      "БД",
			urlT:       "http://localhost:8080/api/shorten/batch",
			methodReqT: http.MethodPost,
			batchLongURLT: []rxLongURLBatchT{
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
			batchLongURLT: []rxLongURLBatchT{
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

			batchShortURL := make([]txShortURLBatchT, 0)

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
		})
	}
}

func Test_internalShortURLFromLongBatch_FAULT(t *testing.T) {

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		BaseAddrShortURL: "http://localhost:8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
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

func Test_InternalDeleteUserURLs_SUCCESS(t *testing.T) {

	conf := &ShortLongT{
		List: &ShortLongURLT{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			Mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDBT{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
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

// Метрики

func Test_UpdateMetricByTypeAndName_SUCCESS(t *testing.T) {

	// подготовка
	conf := &MetricsHandlerT{
		Metrics: &MetricsT{
			GaugeMetrics:   make(map[string]float64),
			CounterMetrics: make(map[string]int64),
			Mu:             sync.RWMutex{},
		},
		DB:                  &MetricsDBT{},
		StoreIntervalMetr:   "300",
		FileStoragePathMetr: "storageMetrics.json",
		RestoreMetr:         "false",
	}

	conf.Metrics.GaugeMetrics["LastGC"] = 1.0
	conf.Metrics.CounterMetrics["counter"] = 1

	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		initMockT      func(mock sqlmock.Sqlmock)
		wantStatusCode int
	}{

		{
			nameT:   "Корректные данные",
			methodT: http.MethodPost,
			urlT:    "http://localhost:8080/update/counter/NumGC/42",
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantStatusCode: http.StatusOK,
		},
	}

	// тесты
	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			req := httptest.NewRequest(tt.methodT, tt.urlT, nil)
			res := httptest.NewRecorder()

			db = nil
			internalUpdateMetricByTypeAndName(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

func Test_UpdateMetricByTypeAndName_FAULT(t *testing.T) {

	// подготовка
	conf := &MetricsHandlerT{
		Metrics: &MetricsT{
			GaugeMetrics:   make(map[string]float64),
			CounterMetrics: make(map[string]int64),
			Mu:             sync.RWMutex{},
		},
		StoreIntervalMetr:   "300",
		FileStoragePathMetr: "storageMetrics.json",
		RestoreMetr:         "false",
	}

	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		wantStatusCode int
		initMockT      func(mock sqlmock.Sqlmock)
		wantBody       string
	}{
		{
			nameT:          "wrong metric type",
			methodT:        http.MethodPost,
			urlT:           "http://localhost:8080/update/wrong/PollCount/1",
			wantStatusCode: http.StatusNotFound,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nameT:          "wrong URL",
			methodT:        http.MethodPost,
			urlT:           "http://localhost:8080/update/counter//1",
			wantStatusCode: http.StatusNotFound,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nameT:          "wrong method",
			methodT:        http.MethodGet,
			urlT:           "http://localhost:8080/update/gauge/PollCount/1",
			wantStatusCode: http.StatusBadRequest,
			initMockT: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("https://practicum.yandex.ru/", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
	}
	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMockT(mock)

			req := httptest.NewRequest(tt.methodT, tt.urlT, nil)
			res := httptest.NewRecorder()

			internalUpdateMetricByTypeAndName(db, conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

func Test_ValueMetricByTypeAndName_SUCCESS(t *testing.T) {

	// подготовка
	conf := &MetricsHandlerT{
		Metrics: &MetricsT{
			GaugeMetrics:   make(map[string]float64),
			CounterMetrics: make(map[string]int64),
			Mu:             sync.RWMutex{},
		},
		StoreIntervalMetr:   "300",
		FileStoragePathMetr: "storageMetrics.json",
		RestoreMetr:         "false",
	}

	conf.Metrics.CounterMetrics["PollCount"] = 123

	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		wantStatusCode int
		wantBody       string
	}{
		{
			nameT:          "корректные данные",
			methodT:        http.MethodGet,
			urlT:           "http://localhost:8080/value/counter/PollCount",
			wantStatusCode: http.StatusOK,
			wantBody:       "123",
		},
	}
	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {
			req := httptest.NewRequest(tt.methodT, tt.urlT, nil)
			res := httptest.NewRecorder()

			internalValueMetricByTypeAndName(conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoErrorf(t, err, "ошибка при чтении тела ответа {%v}", err)
			assert.Equalf(t, tt.wantBody, string(body), "принято{%s} а ожидалось {%s}", tt.wantBody, string(body))
		})
	}
}

func Test_ValueMetricByTypeAndName_FAULT(t *testing.T) {
	// подготовка
	conf := &MetricsHandlerT{
		Metrics: &MetricsT{
			GaugeMetrics:   make(map[string]float64),
			CounterMetrics: make(map[string]int64),
			Mu:             sync.RWMutex{},
		},
		StoreIntervalMetr:   "300",
		FileStoragePathMetr: "storageMetrics.json",
		RestoreMetr:         "false",
	}

	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		wantStatusCode int
		wantBody       string
	}{
		{
			nameT:          "wrong metric type",
			methodT:        http.MethodGet,
			urlT:           "http://localhost:8080/value/wrong/PollCount",
			wantStatusCode: http.StatusNotFound,
		},
		{
			nameT:          "wrong URL",
			methodT:        http.MethodGet,
			urlT:           "http://localhost:8080/value/counter//",
			wantStatusCode: http.StatusNotFound,
		},
		{
			nameT:          "wrong method",
			methodT:        http.MethodPost,
			urlT:           "http://localhost:8080/value/gauge/PollCount",
			wantStatusCode: http.StatusBadRequest,
		},
	}
	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {
			req := httptest.NewRequest(tt.methodT, tt.urlT, nil)
			res := httptest.NewRecorder()

			internalValueMetricByTypeAndName(conf, res, req)

			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			assert.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

func TestAllMetricsHTML_SUCCESS(t *testing.T) {

	testMetrics := NewMetrics()

	testMetrics.CounterMetrics["PollCount"] = 10
	testMetrics.CounterMetrics["SomeCounter"] = 5
	testMetrics.GaugeMetrics["Alloc"] = 123.45
	testMetrics.GaugeMetrics["RandomValue"] = 987.65

	metricsHandler := &MetricsHandlerT{
		Metrics: testMetrics,
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	metricsHandler.AllMetricsHTML(res, req)
	resp := res.Result()
	defer func() {
		err := resp.Body.Close()
		assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
	}()

	require.Equalf(t, http.StatusOK, resp.StatusCode, "ожидался код ответа {%d}, а принят {%d}", http.StatusOK, resp.StatusCode)

	expectedContentType := "text/html; charset=utf-8"
	contentTypeRx := res.Header().Get("Content-Type")
	require.Equalf(t, expectedContentType, contentTypeRx, "ожидается контент {%s}, а принят {%s}", expectedContentType, contentTypeRx)

	body, err := io.ReadAll(resp.Body)
	require.NoErrorf(t, err, "ошибка при чтении тела ответа {%v}", err)

	responseBody := string(body)

	assert.Contains(t, responseBody, "<html>", "HTML нет <html> тега")
	assert.Contains(t, responseBody, "<title>МЕТРИКИ</title>", "HTML нет title")
	assert.Contains(t, responseBody, "<h1>Доступные метрики</h1>", "HTML нет h1 заголовка")
	assert.Contains(t, responseBody, "<h2>Gauge</h2>", "HTML нет Gauge заголовка")
	assert.Contains(t, responseBody, "<h2>Counter</h2>", "HTML нет Counter заголовка")

	assert.Contains(t, responseBody, "<li>Alloc: 123.450000</li>", "HTML нет соответствия Alloc")
	assert.Contains(t, responseBody, "<li>PollCount: 10</li>", "HTML нет соответствия PollCount")
	assert.Contains(t, responseBody, "<li>SomeCounter: 5</li>", "HTML нет соответствия SomeCounter")

	assert.Contains(t, responseBody, "<li>RandomValue: 987.650000</li>", "HTML нет соответствия RandomValue")
	assert.Contains(t, responseBody, "</li>", "HTML нет тега заурытия (</li>)")

}

func TestAllMetricsHTML_FAULT(t *testing.T) {

	testMetrics := NewMetrics()

	metricsHandler := &MetricsHandlerT{
		Metrics: testMetrics,
	}

	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		wantStatusCode int
	}{
		{
			nameT:          "wrong method",
			methodT:        http.MethodPost,
			urlT:           "http://localhost:8080/",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {
			req := httptest.NewRequest(tt.methodT, tt.urlT, nil)
			res := httptest.NewRecorder()

			metricsHandler.AllMetricsHTML(res, req)
			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			assert.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код ответа {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

func Test_MetricByJSON_SUCCESS(t *testing.T) {
	testMetrics := NewMetrics()

	metricsHandler := &MetricsHandlerT{
		Metrics: testMetrics,
	}
	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		body           Metrics
		contentType    string
		valueMetric    float64
		wantStatusCode int
	}{
		{
			nameT:   "данные корректны",
			methodT: http.MethodPost,
			urlT:    "http://localhost:8080/update",
			body: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			contentType:    "application/json",
			valueMetric:    1744184459,
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {

			tt.body.Value = new(float64)
			*tt.body.Value = tt.valueMetric
			metricsHandler.Metrics.GaugeMetrics[tt.body.ID] = *tt.body.Value

			rawData, err := json.Marshal(tt.body)
			require.NoErrorf(t, err, "неожиданная ошибка сериализации <%v>", err)

			txData := bytes.NewBuffer(rawData)

			req := httptest.NewRequest(tt.methodT, tt.urlT, txData)
			res := httptest.NewRecorder()

			req.Header.Set("Content-Type", tt.contentType)

			metricsHandler.MetricByJSON(res, req)
			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			require.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидался код {%d}, а принят {%d}", tt.wantStatusCode, resp.StatusCode)

			var rxData Metrics

			err = json.NewDecoder(res.Body).Decode(&rxData)
			require.NoErrorf(t, err, "неожиданная ошибка десериализации <%v>", err)

			assert.Equalf(t, tt.body.ID, rxData.ID, "ожидался ID <%s> а принят <%s>", tt.body.ID, rxData.ID)
			assert.Equalf(t, tt.body.MType, rxData.MType, "ожидался MType <%s> а принят <%s>", tt.body.MType, rxData.MType)
			assert.Equalf(t, *tt.body.Value, *rxData.Value, "ожидался Value <%f> а принят <%f>", *tt.body.Value, *rxData.Value)

		})
	}
}

func Test_MetricByJSON_FAULT(t *testing.T) {
	testMetrics := NewMetrics()

	metricsHandler := &MetricsHandlerT{
		Metrics: testMetrics,
	}
	testsData := []struct {
		nameT          string
		methodT        string
		urlT           string
		metricsInit    Metrics
		body           Metrics
		contentType    string
		valueMetric    float64
		wantStatusCode int
	}{
		{
			nameT:   "неверный метод",
			methodT: http.MethodGet,
			urlT:    "http://localhost:8080/update",
			metricsInit: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			body: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			contentType:    "application/json",
			valueMetric:    1744184459,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:   "неверный контент",
			methodT: http.MethodPost,
			urlT:    "http://localhost:8080/update",
			metricsInit: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			body: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			contentType:    "application/AAA",
			valueMetric:    1744184459,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:   "нет содержимого в ID",
			methodT: http.MethodPost,
			urlT:    "http://localhost:8080/update",
			metricsInit: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			body: Metrics{
				ID:    "",
				MType: "gauge",
				Value: new(float64),
			},
			contentType:    "application/json",
			valueMetric:    1744184459,
			wantStatusCode: http.StatusBadRequest,
		},
		{
			nameT:   "нет содержимого в MType",
			methodT: http.MethodPost,
			urlT:    "http://localhost:8080/update",
			metricsInit: Metrics{
				ID:    "LastGC",
				MType: "gauge",
				Value: new(float64),
			},
			body: Metrics{
				ID:    "LastGC",
				MType: "",
				Value: new(float64),
			},
			contentType:    "application/json",
			valueMetric:    1744184459,
			wantStatusCode: http.StatusBadRequest,
		},
	}
	for _, tt := range testsData {
		t.Run(tt.nameT, func(t *testing.T) {

			tt.metricsInit.Value = new(float64)
			*tt.metricsInit.Value = tt.valueMetric
			metricsHandler.Metrics.GaugeMetrics[tt.metricsInit.ID] = *tt.metricsInit.Value

			rawData, err := json.Marshal(tt.body)
			require.NoErrorf(t, err, "неожиданная ошибка сериализации <%v>", err)

			txData := bytes.NewBuffer(rawData)

			req := httptest.NewRequest(tt.methodT, tt.urlT, txData)
			res := httptest.NewRecorder()

			req.Header.Set("Content-Type", tt.contentType)

			metricsHandler.MetricByJSON(res, req)
			resp := res.Result()
			defer func() {
				err := resp.Body.Close()
				assert.NoErrorf(t, err, "ошибка при закрытии потока {%v}", err)
			}()

			assert.Equalf(t, tt.wantStatusCode, resp.StatusCode, "ожидася код <%d>, а принят <%d>", tt.wantStatusCode, resp.StatusCode)
		})
	}
}

func Test_storageDBCounterMetrics_SUCCESS(t *testing.T) {

	testsData := []struct {
		nameTest string
		name     string
		value    int64
		initMock func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "Корректные данные",
			name:     "AAA",
			value:    123,
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("AAA", 123).
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

			err = storageDBCounterMetrics(db, tt.name, tt.value)
			require.NoError(t, err)
		})
	}
}

func Test_storageDBCounterMetrics_FAULT(t *testing.T) {

	testsData := []struct {
		nameTest  string
		usePtrDB  bool
		name      string
		value     int64
		initMock  func(mock sqlmock.Sqlmock)
		wantError string
	}{
		{
			nameTest: "Нет указателя на БД",
			usePtrDB: false,
			name:     "AAA",
			value:    123,
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("AAA", 123).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "ошибка сохранения метрики типа counter в БД. В аргументе db нет указателя на БД",
		},
		{
			nameTest: "Нет имени метрики",
			usePtrDB: true,
			name:     "",
			value:    123,
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("AAA", 123).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "ошибка сохранения метрики типа counter в БД. Принято пустое значение name аргумента",
		},
	}

	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMock(mock)

			ptrDB := db
			if !tt.usePtrDB {
				ptrDB = nil
			}

			err = storageDBCounterMetrics(ptrDB, tt.name, tt.value)
			require.Equalf(t, tt.wantError, err.Error(), "ожидалась ошибка <%s>, а принято <%s>", tt.wantError, err.Error())
		})
	}
}

func Test_storageDBGaugeMetrics_SUCCESS(t *testing.T) {

	testsData := []struct {
		nameTest string
		name     string
		value    float64
		initMock func(mock sqlmock.Sqlmock)
	}{
		{
			nameTest: "Корректные данные",
			name:     "AAA",
			value:    123.4,
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("AAA", 123.4).
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

			err = storageDBGaugeMetrics(db, tt.name, tt.value)
			require.NoError(t, err)
		})
	}
}

func Test_storageDBGaugeMetrics_FAULT(t *testing.T) {

	testsData := []struct {
		nameTest  string
		usePtrDB  bool
		name      string
		value     float64
		initMock  func(mock sqlmock.Sqlmock)
		wantError string
	}{
		{
			nameTest: "Нет указателя на БД",
			usePtrDB: false,
			name:     "AAA",
			value:    123,
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("AAA", 123).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "ошибка сохранения метрики типа gauge в БД. В аргументе db нет указателя на БД",
		},
		{
			nameTest: "Нет имени метрики",
			usePtrDB: true,
			name:     "",
			value:    123,
			initMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO").
					WithArgs("AAA", 123).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantError: "ошибка сохранения метрики типа gauge в БД. Принято пустое значение name аргумента",
		},
	}

	for _, tt := range testsData {
		t.Run(tt.nameTest, func(t *testing.T) {

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tt.initMock(mock)

			ptrDB := db
			if !tt.usePtrDB {
				ptrDB = nil
			}

			err = storageDBGaugeMetrics(ptrDB, tt.name, tt.value)
			require.Equalf(t, tt.wantError, err.Error(), "ожидалась ошибка <%s>, а принято <%s>", tt.wantError, err.Error())
		})
	}
}
