// observer пакет реализации паттерна Наблюдатель. Секция с методами объекта.
//
// RegistrationObserver - дабавление наблюдателя.
// UnRegistrationObserver - удаление наблюдателя.
// Notify - Оповещение.
package observer

import (
	"encoding/json"

	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	"go.uber.org/zap"
)

// RegistrationObserver регистрация наблюдателя.
//
// Парметры:
//
//	o - интерфейс наблюдателя.
func (s *source) RegistrationObserver(o ActionsObservers) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if s.obs == nil {
		s.obs = make(map[string]ActionsObservers)
	}
	s.obs[o.GetID()] = o
	logger.Log.Info("Зарегистрирован наблюдатель", zap.String("obsID", o.GetID()))
}

// UnRegistrationObserver удаляет наблюдателя.
//
// Парметры:
//
//	o - интерфейс наблюдателя.
func (s *source) UnRegistrationObserver(o ActionsObservers) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	delete(s.obs, o.GetID())
	logger.Log.Info("Удалён наблюдатель", zap.String("obsID", o.GetID()))
}

// Notify вызов оповещений наблюдателей.
//
// Парметры:
//
//	msg - сообщение оповещения.
func (s *source) Notify(msg AuditEvent) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	// Отправка
	for _, os := range s.obs {
		if err := os.SendMsg(msg); err != nil {

			jsonData, jsonErr := json.Marshal(msg)
			if jsonErr != nil {
				logger.Log.Error("ошибка json.Marshal",
					zap.Error(jsonErr),
				)
				continue
			}

			logger.Log.Error("ошибка оповещения наблюдателя",
				zap.String("obsID", os.GetID()),
				zap.Error(err),
				zap.String("msg", string(jsonData)),
			)
			continue
		}
		logger.Log.Info("Наблюдателю передано оповещение", zap.String("obsID", os.GetID()))
	}
}
