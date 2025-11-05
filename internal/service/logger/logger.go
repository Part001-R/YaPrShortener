// logger пакет.
// Функция Initialize, реализует создание экземпляра.
package logger

import (
	"sync"

	"go.uber.org/zap"
)

// Экземпляр логгера.
var log *zap.Logger = zap.NewNop()

// Обеспечение единоразовой инициализации.
var once sync.Once

// NewLogger инициализирует логгер. Возвращается логгер и ошибка.
//
// Параметры:
//
//	level - уровень логирования.
func NewLogger(level string) (*zap.Logger, error) {

	var initErr error

	once.Do(func() {
		// Преобразуем текстовый уровень логирования в zap.AtomicLevel.
		lvl, err := zap.ParseAtomicLevel(level)
		if err != nil {
			initErr = err
			return
		}
		// Создаем новую конфигурацию логера.
		cfg := zap.NewProductionConfig()
		// Устанавливаем уровень.
		cfg.Level = lvl
		// Создаем логер на основе конфигурации.
		zl, err := cfg.Build()
		if err != nil {
			initErr = err
			return
		}
		// Устанавливаем синглтон.
		log = zl
	})

	if initErr != nil {
		return nil, initErr
	}
	return log, nil
}
