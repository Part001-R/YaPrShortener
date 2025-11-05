// observer пакет реализации паттерна Наблюдатель. Секция с типами данных.
// Содержит конструктор - NewObserver.
package observer

import (
	"sync"

	"go.uber.org/zap"
)

// Сообщение аудита.
type AuditEvent struct {
	Timestamp int64  `json:"ts"`                // unix timestamp события.
	Action    string `json:"action"`            // действие: shorten (создание) или follow (прохождение по ссылке).
	UserID    string `json:"user_id,omitempty"` // идентификатор пользователя, если есть.
	URL       string `json:"url"`               // оригинальный (не сокращённый) URL.
}

// Источник.
type source struct {
	obs map[string]ActionsObservers
	mtx sync.RWMutex
	log *zap.Logger
}

// Интерфейс для взаимодействия с пакетом.
type Action interface {
	RegistrationObserver(o ActionsObservers)
	UnRegistrationObserver(o ActionsObservers)
	Notify(msg AuditEvent)
}

// Экземпляр источника оповещений.
var obsSrc *source

// Обеспечение единоразовой инициализации.
var once sync.Once

// Конструктор.
func NewObserver(log *zap.Logger) Action {
	once.Do(func() {
		obsSrc = &source{
			obs: map[string]ActionsObservers{},
			mtx: sync.RWMutex{},
			log: log,
		}
	})

	return obsSrc
}
