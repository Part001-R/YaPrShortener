// logger пакет.
// Функция Initialize, реализует создание экземпляра.
package logger

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

var (
	// Экземпляр логгера.
	log *zap.Logger = zap.NewNop()

	// Обеспечение единоразовой инициализации.
	once sync.Once

	// Поддерживаемые уровни логирования
	validLevels = map[string]struct{}{
		"debug": {},
		"info":  {},
		"warn":  {},
		"error": {},
		"Debug": {},
		"Info":  {},
		"Warn":  {},
		"Error": {},
	}
)

// NewLogger инициализирует логгер. Возвращается логгер и ошибка.
//
// Параметры:
//
//	level - уровень логирования.
func NewLogger(level string) (*zap.Logger, error) {

	var initErr error

	// Проверка
	if _, exists := validLevels[level]; !exists {
		return nil, fmt.Errorf("неподдерживаемый уровень логирования: %s", level)
	}

	once.Do(func() {
		// Преобразуем текстовый уровень логирования в zap.AtomicLevel.
		lvl := zap.NewAtomicLevel()
		lvl.UnmarshalText([]byte(level))

		// Создаем новую конфигурацию логера.
		cfg := zap.NewProductionConfig()

		// Устанавливаем уровень.
		cfg.Level = lvl

		// Создаем логгер на основе конфигурации.
		var err error
		log, err = cfg.Build()
		if err != nil {
			initErr = err
			return
		}
	})

	if initErr != nil {
		return nil, initErr
	}
	return log, nil
}
