// service, тесты.
package service

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetValueOrDefault, тест.
func Test_GetValueOrDefault_SUCCESS(t *testing.T) {

	// Есть передаваемой значение.
	valueTx := "AAA"
	valueRx := GetValueOrDefault(valueTx)
	assert.Equalf(t, valueTx, valueRx, "ожидалось <%s>, а принято <%s>", valueTx, valueRx)

	// Нет передаваемого значения.
	want := "N/A"
	valueTx = ""
	valueRx = GetValueOrDefault(valueTx)
	assert.Equalf(t, want, valueRx, "ожидалось <%s>, а принято <%s>", want, valueRx)
}

// prepare, тест. Проверяется отработка подготовительных действий, при запуске приложения.
func Test_prepare_SUCCESS(t *testing.T) {

	os.Args = []string{}
	os.Args = []string{"cmd", "-a=:9998", "-b=http://localhost:5500/", "-l=info", "-f=test.json", "-s=true"}

	params, err := prepare()
	require.NoErrorf(t, err, "ошибка в функции prepare: <%v>", err)

	// Проверка флагов.
	assert.Equalf(t, ":9998", params.flags.Port, "У Port ожидался <%s>, а принято <%s>", ":9999", params.flags.Port)
	assert.Equalf(t, "http://localhost:5500/", params.flags.BaseAddrShortURL, "У BaseAddrShortURL ожидался <%s>, а принято <%s>", "http://localhost:5500/", params.flags.BaseAddrShortURL)
	assert.Equalf(t, "info", params.flags.LogLevel, "У LogLevel ожидался <%s>, а принято <%s>", "info", params.flags.LogLevel)
	assert.Equalf(t, "true", params.flags.EnableHTTPS, "У EnableHTTPS ожидался <%s>, а принято <%s>", "true", params.flags.EnableHTTPS)

	// Проверка логгера.
	assert.NotNil(t, params.log, "log должен быть инициализирован и не равен nil")

	// Проверка интерфейса in memory хранилища.
	assert.NotNil(t, params.storageLongShort, "storageLongShort должен быть инициализирован и не равен nil")

	// Удаление созданного файла.
	err = os.Remove("test.json")
	assert.NoErrorf(t, err, "ошибка при удалении файла: <%v>", err)
}

// server, тест. Проверяется запуск HTTP.
func Test_RunHTTP_SUCCESS(t *testing.T) {

	os.Args = []string{}

	os.Args = []string{"cmd", "-a=:9997", "-b=http://localhost:5500/", "-l=info", "-f=test.json", "-s=false"}

	// Запуск
	go func() {
		err := Run()
		require.NoError(t, err, "ошибка в функции Run")
	}()
	time.Sleep(1 * time.Second) // Время на запуск

	// Канал для принятия сигналов
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Сигнал на остановку
	signalChan <- syscall.SIGINT
	time.Sleep(1 * time.Second) // Время на остановку

	// Действия после остановки
	err := os.Remove("test.json")
	require.NoErrorf(t, err, "ошибка при удалении файла: <%v>", err)
}

// startUpHTTPSServer.
func Test_startUpHTTPServer_SUCCESS(t *testing.T) {

	// Подготовка.
	log, err := logger.NewLogger("debug")
	require.NoErrorf(t, err, "ошибка при создании логгера:<%v>", err)

	cr := chi.NewRouter()

	srvConf := &http.Server{
		Addr:    ":4488",
		Handler: cr,
	}

	// Сигналы остановки.
	chSrvErr := make(chan error)

	// Запуск функции.
	go startUpHTTPServer(srvConf, chSrvErr, log)

	select {
	case <-time.After(2 * time.Second):

	case err := <-chSrvErr:
		require.NoErrorf(t, err, "ошибка при запуске сервера:<%v>", err)
	}
}
