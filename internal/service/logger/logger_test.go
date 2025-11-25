// logger, тесты.
package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewLogger, тесты.
func Test_NewLogger(t *testing.T) {

	// Данные для тестов
	testsData := []struct {
		level       string
		expectError bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"wrongLevel", true},
	}

	// Тесты
	for _, test := range testsData {
		t.Run(test.level, func(t *testing.T) {
			logger, err := NewLogger(test.level)

			if test.expectError {
				require.Error(t, err, "ожидалась ошибка, но не сформирована: %s", test.level)
				assert.Nil(t, logger, "указатель должен отсутствовать")
				return
			}

			require.NoError(t, err, "неожиданная ошибка:<%v>, для уровня:<%s>", err, test.level)
			assert.NotNil(t, logger, "указатель должен присутствовать")
		})
	}
}
