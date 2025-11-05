package observerfile

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SendMsg_SUCCESS(t *testing.T) {

	type AuditEvent struct {
		Timestamp int64  `json:"ts"`                // unix timestamp события.
		Action    string `json:"action"`            // действие: shorten (создание) или follow (прохождение по ссылке).
		UserID    string `json:"user_id,omitempty"` // идентификатор пользователя, если есть.
		URL       string `json:"url"`               // оригинальный (не сокращённый) URL.
	}

	// Подготовка тестового экземпляра obsFile.
	obsFileHandler := &obsFile{
		filePath: "test_audit_log.json",
	}

	tn := time.Now().Unix()

	msg := AuditEvent{
		Timestamp: tn,
		Action:    "foo",
		UserID:    "",
		URL:       "http://bar.com",
	}

	msg2 := AuditEvent{
		Timestamp: tn,
		Action:    "foo",
		UserID:    "bar",
		URL:       "http://bar.com",
	}

	// Данные для тестов.
	dataTest := []struct {
		nameT   string
		msgT    AuditEvent
		wantMsg string
	}{
		{
			nameT:   "Нет UserID",
			msgT:    msg,
			wantMsg: fmt.Sprintf(`{"ts":%d,"action":"%s","url":"%s"}`, tn, msg.Action, msg.URL),
		},
		{
			nameT:   "Есть UserID",
			msgT:    msg2,
			wantMsg: fmt.Sprintf(`{"ts":%d,"action":"%s","user_id":"%s","url":"%s"}`, tn, msg2.Action, msg2.UserID, msg2.URL),
		},
	}

	// Тесты.
	for _, tt := range dataTest {
		t.Run(tt.nameT, func(t *testing.T) {

			// Открытие файла.
			file, err := os.OpenFile(obsFileHandler.filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			require.NoErrorf(t, err, "неожиданная ошибка при открытии файла <%v>", err)

			defer func() {
				if err := file.Close(); err != nil {
					require.NoErrorf(t, err, "неожиданная ошибка при закрытиии файла: <%v>", err)
				}

				if err := os.Remove(obsFileHandler.filePath); err != nil {
					require.NoErrorf(t, err, "неожиданная ошибка при удалении файла: <%v>", err)
				}

			}()

			// Передача данных в файл.
			err = obsFileHandler.SendMsg(observer.AuditEvent(tt.msgT))
			require.NoErrorf(t, err, "ошибка при отправке сообщения <%v>", err)

			// Чтение файла.
			data, err := os.ReadFile(obsFileHandler.filePath)
			require.NoErrorf(t, err, "ошибка при чтении файла <%v>", err)

			// Проверка результата.
			rxStr := strings.TrimSpace(string(data))
			assert.Equalf(t, tt.wantMsg, rxStr, "некорректный URL: ожидалось <%s>, получено <%s>", tt.wantMsg, rxStr)

		})
	}
}

func Test_GetID_SUCCESS(t *testing.T) {

	filePath := "./foo.json"
	fileName := "file.txt"
	obsFile := NewObserverFile(fileName, filePath, nil)

	rxName := obsFile.GetID()
	require.Equalf(t, fileName, rxName, "ожидалось <%s>, а принято <%s>", fileName, rxName)
}
