package servicerpc

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Part001-R/YaPrShortener/internal/handler"
	"github.com/Part001-R/YaPrShortener/internal/service/flags"
	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	pb "github.com/Part001-R/YaPrShortener/proto/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// ShortenURL.
func Test_ShortenURL_RPC_SUCCESS(t *testing.T) {

	//
	// Подготовка.
	//

	// Экземляр in memory
	storage := &handler.ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
	}

	// БД
	// Создание мок-объекта БД
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ok := handler.ResetNewShortenerDB() // Так как инициализация через sync.Once (функция работает только в файлах *_test.go)
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	instDB := handler.NewShortenerDB(db)

	// Флаги
	fl := &flags.Config{
		Port:             ":8080",
		BaseAddrShortURL: "http://localhost:8080/",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "",
		EnableHTTPS:      "false",
		ConfigFile:       "",
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Подготовка экземпляра.
	ok = handler.ResetNewShortener() // Так как инициализация через sync.Once
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	actionsWork := handler.NewInstWorkFunc()

	inst := handler.NewShortenerActions(storage, instDB, fl, obsSrc, log, actionsWork)
	assert.NotNil(t, inst, "отсутствует указатель")

	conf := handler.ShortenerForRPC() // Получение указателя на конфигурацию.
	assert.NotNil(t, conf, "отсутствует указатель")

	conf.DB.Ptr = nil

	//
	// Логика теста.
	//

	instRPC := ShortenerService{
		Conf:    conf,
		Actions: actionsWork,
	}

	// Добавление метаданных.
	md := metadata.Pairs("authorization", "Foo")
	ctxMd := metadata.NewIncomingContext(context.Background(), md)

	// Запрос.
	longURL := "https://practicum.yandex.ru"
	req := &pb.URLShortenRequest{Url: longURL}

	// Вызов обработчика.
	res, err := instRPC.ShortenURL(ctxMd, req)
	require.NoError(t, err, "неожиданная ошибка в вызове ShortenURL: <%v>", err)

	// Проверка.
	short, ok := conf.List.ShorByLong[longURL]
	require.Equalf(t, true, ok, "в мапе ShorByLong нет ключа: <%s>", longURL)

	long, ok := conf.List.LongByShort[short]
	require.Equalf(t, true, ok, "в мапе LongByShort нет ключа: <%s>", short)

	assert.Equalf(t, longURL, long, "ожидалось long:<%s>, а принято:<%s>", longURL, long)

	fullShort := conf.BaseAddrShortURL + short
	assert.Equalf(t, fullShort, res.Result, "ожидалось short:<%s>, а принято:<%s>", short, res.Result)

	// Удаление файла.
	os.Remove("storage.json")
}

func Test_ShortenURL_RPC_FAULT(t *testing.T) {

	//
	// Подготовка.
	//

	// Экземляр in memory
	storage := &handler.ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
	}

	// БД
	// Создание мок-объекта БД
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ok := handler.ResetNewShortenerDB() // Так как инициализация через sync.Once (функция работает только в файлах *_test.go)
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	instDB := handler.NewShortenerDB(db)

	// Флаги
	fl := &flags.Config{
		Port:             ":8080",
		BaseAddrShortURL: "http://localhost:8080/",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "",
		EnableHTTPS:      "false",
		ConfigFile:       "",
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Подготовка экземпляра.
	ok = handler.ResetNewShortener() // Так как инициализация через sync.Once
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	actionsWork := handler.NewInstWorkFunc()

	inst := handler.NewShortenerActions(storage, instDB, fl, obsSrc, log, actionsWork)
	assert.NotNil(t, inst, "отсутствует указатель")

	conf := handler.ShortenerForRPC() // Получение указателя на конфигурацию.
	assert.NotNil(t, conf, "отсутствует указатель")

	conf.DB.Ptr = nil

	testData := []struct {
		nameTest string
		mdKey    string
		mdValue  string
		mdUse    bool
		wantErr  string
	}{
		{
			nameTest: "Неверные метаданные",
			mdKey:    "Foo",
			mdValue:  "Bar",
			mdUse:    true,
			wantErr:  "rpc error: code = Unavailable desc = нет данных в authorization",
		},
		{
			nameTest: "Нет метаданных",
			mdKey:    "",
			mdValue:  "",
			mdUse:    false,
			wantErr:  "rpc error: code = Unavailable desc = отсутствуют метаданные",
		},
	}

	//
	// Логика теста.
	//
	for _, tt := range testData {
		t.Run(tt.nameTest, func(t *testing.T) {

			instRPC := ShortenerService{
				Conf:    conf,
				Actions: actionsWork,
			}

			if tt.mdUse {
				// Добавление метаданных.
				md := metadata.Pairs(tt.mdKey, tt.mdValue)
				ctxMd := metadata.NewIncomingContext(context.Background(), md)

				// Запрос.
				longURL := "https://practicum.yandex.ru"
				req := &pb.URLShortenRequest{Url: longURL}

				// Вызов обработчика.
				_, err := instRPC.ShortenURL(ctxMd, req)
				require.Equalf(t, tt.wantErr, err.Error(), "ожидалась ошибка:<%v>, а принято:<%s>", tt.wantErr, err.Error())
			}

			if !tt.mdUse {
				// Запрос.
				longURL := "https://practicum.yandex.ru"
				req := &pb.URLShortenRequest{Url: longURL}

				// Вызов обработчика.
				_, err := instRPC.ShortenURL(context.Background(), req)
				require.Equalf(t, tt.wantErr, err.Error(), "ожидалась ошибка:<%v>, а принято:<%s>", tt.wantErr, err.Error())
			}

		})
	}
}

// ExpandURL.
func Test_ExpandURL_RPC(t *testing.T) {

	//
	// Подготовка.
	//

	// Экземляр in memory
	storage := &handler.ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
	}

	// БД
	// Создание мок-объекта БД
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ok := handler.ResetNewShortenerDB() // Так как инициализация через sync.Once (функция работает только в файлах *_test.go)
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	instDB := handler.NewShortenerDB(db)

	// Флаги
	fl := &flags.Config{
		Port:             ":8080",
		BaseAddrShortURL: "http://localhost:8080/",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "",
		EnableHTTPS:      "false",
		ConfigFile:       "",
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Подготовка экземпляра.
	ok = handler.ResetNewShortener() // Так как инициализация через sync.Once
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	actionsWork := handler.NewInstWorkFunc()

	inst := handler.NewShortenerActions(storage, instDB, fl, obsSrc, log, actionsWork)
	assert.NotNil(t, inst, "отсутствует указатель")

	conf := handler.ShortenerForRPC() // Получение указателя на конфигурацию.
	assert.NotNil(t, conf, "отсутствует указатель")

	conf.DB.Ptr = nil

	//
	// Инициализация мап.
	//

	short := "EwHXdJfB"
	long := "https://practicum.yandex.ru/"

	conf.List.LongByShort[short] = long
	conf.List.ShorByLong[long] = short

	//
	// Логика теста.
	//

	instRPC := ShortenerService{
		Conf:    conf,
		Actions: actionsWork,
	}

	req := &pb.URLExpandRequest{Id: short}

	res, err := instRPC.ExpandURL(context.Background(), req)
	require.NoErrorf(t, err, "неожиданная ошибка:<%v>", err)
	assert.Equalf(t, long, res.Result, "ожидалось:<%s>, а принято:<%s>", long, res.Result)
}

// ListUserURLs.
func Test_ListUserURLs_RPC(t *testing.T) {

	//
	// Подготовка.
	//

	// Экземляр in memory
	storage := &handler.ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
	}

	// БД
	// Создание мок-объекта БД
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ok := handler.ResetNewShortenerDB() // Так как инициализация через sync.Once (функция работает только в файлах *_test.go)
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	instDB := handler.NewShortenerDB(db)

	// Флаги
	fl := &flags.Config{
		Port:             ":8080",
		BaseAddrShortURL: "http://localhost:8080/",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "",
		EnableHTTPS:      "false",
		ConfigFile:       "",
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Подготовка экземпляра.
	ok = handler.ResetNewShortener() // Так как инициализация через sync.Once
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	actionsWork := handler.NewInstWorkFunc()

	inst := handler.NewShortenerActions(storage, instDB, fl, obsSrc, log, actionsWork)
	assert.NotNil(t, inst, "отсутствует указатель")

	conf := handler.ShortenerForRPC() // Получение указателя на конфигурацию.
	assert.NotNil(t, conf, "отсутствует указатель")

	conf.DB.Ptr = nil

	//
	// Инициализация мап.
	//

	short1 := "Foo"
	long1 := "https://Foo.ru/"
	short2 := "Bar"
	long2 := "https://Bar.ru/"

	conf.List.LongByShort[short1] = long1
	conf.List.ShorByLong[long1] = short1
	conf.List.LongByShort[short2] = long2
	conf.List.ShorByLong[long2] = short2

	//
	// Логика теста.
	//

	instRPC := ShortenerService{
		Conf:    conf,
		Actions: actionsWork,
	}

	res, err := instRPC.ListUserURLs(context.Background(), nil)
	require.NoErrorf(t, err, "неожиданная ошибка:<%v>", err)

	// Ожидаемый результат
	want := &pb.UserURLsResponse{
		Url: []*pb.URLData{
			{
				ShortUrl:    conf.BaseAddrShortURL + short1,
				OriginalUrl: long1,
			},
			{
				ShortUrl:    conf.BaseAddrShortURL + short2,
				OriginalUrl: long2,
			},
		},
	}

	require.NotNil(t, res, "ожидался ответ, а принято nil")
	assert.Equalf(t, want, res, "в ответе ожидалось:<%v>, а принято:<%v>", want, res)
}

// Комплексная проверка всех трёх обработчиков
func Test_Full(t *testing.T) {

	//
	// Подготовка.
	//

	// Экземляр in memory
	storage := &handler.ShortLongURL{
		ShorByLong:  map[string]string{},
		LongByShort: map[string]string{},
	}

	// БД
	// Создание мок-объекта БД
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ok := handler.ResetNewShortenerDB() // Так как инициализация через sync.Once (функция работает только в файлах *_test.go)
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	instDB := handler.NewShortenerDB(db)

	// Флаги
	fl := &flags.Config{
		Port:             ":8080",
		BaseAddrShortURL: "http://localhost:8080/",
		LogLevel:         "debug",
		FileStoragePath:  "storage.json",
		AuditFile:        "",
		AuditURL:         "",
		DSNDB:            "",
		EnableHTTPS:      "false",
		ConfigFile:       "",
		TrustedSubnet:    "127.0.0.0/8",
	}

	// Конструктор логгера.
	log, err := logger.NewLogger("debug")
	require.NoErrorf(t, err, "неожиданная ошибка при создании логгера: <%v>", err)

	// Конструктор наблюдателя.
	obsSrc := observer.NewObserver(log)

	// Подготовка экземпляра.
	ok = handler.ResetNewShortener() // Так как инициализация через sync.Once
	require.Equalf(t, true, ok, "сброс не выполнен: <%t>", ok)

	actionsWork := handler.NewInstWorkFunc()

	inst := handler.NewShortenerActions(storage, instDB, fl, obsSrc, log, actionsWork)
	assert.NotNil(t, inst, "отсутствует указатель")

	conf := handler.ShortenerForRPC() // Получение указателя на конфигурацию.
	assert.NotNil(t, conf, "отсутствует указатель")

	conf.DB.Ptr = nil

	//
	// Передача длинного URL и формирование короткого представления.
	//

	instRPC := ShortenerService{
		Conf:    conf,
		Actions: actionsWork,
	}

	// Добавление метаданных.
	md := metadata.Pairs("authorization", "Foo")
	ctxMd := metadata.NewIncomingContext(context.Background(), md)

	// Запрос.
	longURL := "https://practicum.yandex.ru"
	req := &pb.URLShortenRequest{Url: longURL}

	// Вызов обработчика.
	res, err := instRPC.ShortenURL(ctxMd, req)
	require.NoError(t, err, "неожиданная ошибка в вызове ShortenURL: <%v>", err)

	shortRx1 := res.Result // Фиксация результата

	//
	// Передача короткого представления и получение длинного URL.
	//

	data1 := strings.Split(shortRx1, "/")
	short1 := data1[len(data1)-1] // выделение short из ответа

	req1 := &pb.URLExpandRequest{Id: short1}

	res1, err := instRPC.ExpandURL(context.Background(), req1)
	require.NoErrorf(t, err, "неожиданная ошибка в вывзове ExpandURL:<%v>", err)

	//
	// Проверка результатов отработки двух предыдущих запросов.
	//

	assert.Equalf(t, longURL, res1.Result, "ожидалось:<%s>, а принято:<%s>", longURL, res1.Result)

	//
	// Запрос пар соответствий.
	//

	res2, err := instRPC.ListUserURLs(context.Background(), nil)
	require.NoErrorf(t, err, "неожиданная ошибка:<%v>", err)

	// Ожидаемый результат
	want := &pb.UserURLsResponse{
		Url: []*pb.URLData{
			{
				ShortUrl:    conf.BaseAddrShortURL + short1,
				OriginalUrl: longURL,
			},
		},
	}

	require.NotNil(t, res, "ожидался ответ, а принято nil")
	assert.Equalf(t, want, res2, "в ответе ожидалось:<%v>, а принято:<%v>", want, res)

	//
	// Завершение.
	//

	err = os.Remove("storage.json")
	assert.NoErrorf(t, err, "неожиданная ошибка при удалении файла:<%v>", err)
}
