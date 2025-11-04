package obstest

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"github.com/Part001-R/YaPrShortener/internal/service/observer/observerfile"
	"github.com/Part001-R/YaPrShortener/internal/service/observer/observerurl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_RegistrationObserver_SUCCESS тест добавления наблюдателя.
func Test_RegistrationObserver_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Наблюдатели.
	obsFileID := "file"
	obsFilePath := "./foo.json"
	obsFile := observerfile.NewObserverFile(obsFileID, obsFilePath, nil)

	obsHTTPID := "HTTP"
	obsHTTPURL := "http://foo.bar"
	obsURL := observerurl.NewObserverURL(obsHTTPID, obsHTTPURL)

	// Источник.
	obsSrc := observer.NewObserver(log)

	// Регистрация наблюдателей.
	obsSrc.RegistrationObserver(obsFile)
	obsSrc.RegistrationObserver(obsURL)

	// Проверка регистрации.
	rxObsFileID := obsFile.GetID()
	assert.Equalf(t, obsFileID, rxObsFileID, "ожидалось <%s>, а принято <%s>", obsFileID, rxObsFileID)

	rxObsHTTPID := obsURL.GetID()
	assert.Equalf(t, obsHTTPID, rxObsHTTPID, "ожидалось <%s>, а принято <%s>", obsHTTPID, rxObsHTTPID)
}

// Test_UnRegistrationObserver_SUCCESS тест удаления наблюдателя.
func Test_UnRegistrationObserver_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Наблюдатели.
	obsFileID := "file"
	obsFilePath := "./foo.json"
	obsFile := observerfile.NewObserverFile(obsFileID, obsFilePath, nil)

	// Источник.
	obsSrc := observer.NewObserver(log)

	// Регистрация наблюдателей.
	obsSrc.RegistrationObserver(obsFile)

	// Передача события.
	file, err := os.OpenFile(obsFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	require.NoErrorf(t, err, "неожиданная ошибка при открытии файла <%v>", err)

	defer func() {
		if err := file.Close(); err != nil {
			require.NoErrorf(t, err, "неожиданная ошибка при закрытиии файла: <%v>", err)
		}

		if err := os.Remove(obsFilePath); err != nil {
			require.NoErrorf(t, err, "неожиданная ошибка при удалении файла: <%v>", err)
		}
	}()

	type AuditEvent struct {
		Timestamp int64  `json:"ts"`                // unix timestamp события.
		Action    string `json:"action"`            // действие: shorten (создание) или follow (прохождение по ссылке).
		UserID    string `json:"user_id,omitempty"` // идентификатор пользователя, если есть.
		URL       string `json:"url"`               // оригинальный (не сокращённый) URL.
	}

	tn := time.Now().Unix()

	msg := AuditEvent{
		Timestamp: tn,
		Action:    "foo",
		UserID:    "",
		URL:       "http://bar.com",
	}

	obsSrc.Notify(observer.AuditEvent(msg))

	// Проверка записи данных в файл.
	data, err := os.ReadFile(obsFilePath)
	require.NoErrorf(t, err, "ошибка при чтении файла <%v>", err)

	wantMsg := fmt.Sprintf(`{"ts":%d,"action":"%s","url":"%s"}`, tn, msg.Action, msg.URL)

	rxStr := strings.TrimSpace(string(data))
	assert.Equalf(t, wantMsg, rxStr, "некорректный URL: ожидалось <%s>, получено <%s>", wantMsg, rxStr)

	// Удаление наблюдателя.
	obsSrc.UnRegistrationObserver(obsFile)

	// Передача второго сообщения.
	// Сообщение не должно быть передано, т.к. выполнено удаление.
	obsSrc.Notify(observer.AuditEvent(msg))

	// Проверка записи данных в .
	data, err = os.ReadFile(obsFilePath)
	require.NoErrorf(t, err, "ошибка при чтении файла <%v>", err)

	rxStr = strings.TrimSpace(string(data))
	assert.Equalf(t, wantMsg, rxStr, "некорректный URL: ожидалось <%s>, получено <%s>", wantMsg, rxStr)
}

// Test_Notify_SUCCESS тест оповещения наблюдателей.
func Test_Notify_SUCCESS(t *testing.T) {

	log, err := logger.NewLogger("Debug")
	require.NoErrorf(t, err, "неожтданная ошибка при создании логгера: <%v>", err)

	// Наблюдатели.
	obsFileID := "file"
	obsFilePath := "./foo.json"
	obsFile := observerfile.NewObserverFile(obsFileID, obsFilePath, nil)

	// Источник.
	obsSrc := observer.NewObserver(log)

	// Регистрация наблюдателей.
	obsSrc.RegistrationObserver(obsFile)

	// Передача события.
	file, err := os.OpenFile(obsFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	require.NoErrorf(t, err, "неожиданная ошибка при открытии файла <%v>", err)

	defer func() {
		if err := file.Close(); err != nil {
			require.NoErrorf(t, err, "неожиданная ошибка при закрытиии файла: <%v>", err)
		}

		if err := os.Remove(obsFilePath); err != nil {
			require.NoErrorf(t, err, "неожиданная ошибка при удалении файла: <%v>", err)
		}
	}()

	type AuditEvent struct {
		Timestamp int64  `json:"ts"`                // unix timestamp события.
		Action    string `json:"action"`            // действие: shorten (создание) или follow (прохождение по ссылке).
		UserID    string `json:"user_id,omitempty"` // идентификатор пользователя, если есть.
		URL       string `json:"url"`               // оригинальный (не сокращённый) URL.
	}

	tn := time.Now().Unix()

	msg := AuditEvent{
		Timestamp: tn,
		Action:    "foo",
		UserID:    "",
		URL:       "http://bar.com",
	}

	// Оповещение.
	obsSrc.Notify(observer.AuditEvent(msg))

	// Проверка записи данных в файл.
	data, err := os.ReadFile(obsFilePath)
	require.NoErrorf(t, err, "ошибка при чтении файла <%v>", err)

	wantMsg := fmt.Sprintf(`{"ts":%d,"action":"%s","url":"%s"}`, tn, msg.Action, msg.URL)

	rxStr := strings.TrimSpace(string(data))
	assert.Equalf(t, wantMsg, rxStr, "некорректный URL: ожидалось <%s>, получено <%s>", wantMsg, rxStr)
}
