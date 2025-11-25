// db, тесты пакета.
package db

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Проверка функции workDir.
func Test_workDir_SUCCESS(t *testing.T) {

	_, err := workDir()
	require.NoErrorf(t, err, "неожиданная ошибка при определении рабочей директории: <v>", err)

}

// Проверка функции ConnectDB.
func Test_ConnectDB_FAULT(t *testing.T) {

	testData := []struct {
		testName    string
		dsn         string
		log         *zap.Logger
		mockPingErr error
		expectError bool
	}{
		{
			testName:    "Пустой DSN",
			dsn:         "",
			log:         nil,
			expectError: true,
		},
		{
			testName:    "Log nil",
			dsn:         "user=username password=password dbname=mydb host=localhost sslmode=disable",
			log:         nil,
			expectError: true,
		},
		{
			testName:    "Ошибка ping",
			dsn:         "user=username password=password dbname=mydb host=localhost sslmode=disable",
			log:         zap.NewNop(),
			mockPingErr: sql.ErrConnDone,
			expectError: true,
		},
	}

	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			// Включение мониторинга Ping
			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)
			defer db.Close()

			if !tt.expectError || tt.mockPingErr != nil {
				mock.ExpectPing().WillReturnError(tt.mockPingErr)
			}

			// Запуск функции
			resDB, closeDB, err := ConnectDB(tt.dsn, tt.log)

			// Проверки
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resDB)
				assert.Nil(t, closeDB)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resDB)
				assert.NotNil(t, closeDB)
				closeDB()
			}

			err = mock.ExpectationsWereMet()
			if err != nil && !tt.expectError {
				t.Errorf("не все ожидания mock были выполнены: %v", err)
			}
		})
	}
}
