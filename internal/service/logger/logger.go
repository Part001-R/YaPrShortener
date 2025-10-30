// logger пакет.
// Функция Initialize, реализует создание экземпляра.
package logger

import (
	"sync"

	"go.uber.org/zap"
)

// Глобальный логгер.
var Log *zap.Logger = zap.NewNop()

// Обеспечение единоразовой инициализации.
var once sync.Once

// Initialize инициализирует логгер. Возвращается ошибка.
//
// Параметры:
//
//	level - уровень логирования.
func Initialize(level string) error {

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
		Log = zl
	})

	return initErr
}
