package observerurl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SendMsg_SUCCESS(t *testing.T) {
	type AuditEvent struct {
		Timestamp int64  `json:"ts"`
		Action    string `json:"action"`
		UserID    string `json:"user_id,omitempty"`
		URL       string `json:"url"`
	}

	tn := time.Now().Unix()

	msg1 := AuditEvent{
		Timestamp: tn,
		Action:    "create",
		UserID:    "",
		URL:       "http://example.com",
	}

	msg2 := AuditEvent{
		Timestamp: tn,
		Action:    "follow",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	// Мок сервер.
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		var payload AuditEvent
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Проверка.
		if payload.UserID == "" {
			assert.Equal(t, payload, msg1)
		} else {
			assert.Equal(t, payload, msg2)
		}

		w.WriteHeader(http.StatusOK)
	})

	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Запуск тестового HTTP сервера.
	server := httptest.NewServer(mux)
	defer server.Close()

	obsURL := &obsURL{
		name:       "HTTP",
		pathURL:    server.URL + "/test",
		clientHTTP: client,
	}

	// Тестирование первого сообщения.
	err := obsURL.SendMsg(observer.AuditEvent(msg1))
	require.NoError(t, err, "ошибка при отправке первого сообщения <%v>", err)

	// Тестирование второго сообщения.
	err = obsURL.SendMsg(observer.AuditEvent(msg2))
	require.NoError(t, err, "ошибка при отправке второго сообщения <%v>", err)
}

func Test_GetID_SUCCESS(t *testing.T) {

	obsID := "HTTP"
	obsURL := "http://bar.com"
	obsFile := NewObserverURL(obsID, obsURL)

	rxName := obsFile.GetID()
	require.Equalf(t, obsID, rxName, "ожидалось <%s>, а принято <%s>", obsID, rxName)
}
