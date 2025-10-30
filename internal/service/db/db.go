// db пакет, реализует взаимодействие с БД, при запуске приложения.
//
// MigrationUpDB - реализация Up миграции.
// workDir - определение рабочей директории.
// ConnectDB - подключение к БД.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Part001-R/YaPrShortener/internal/service/logger"
	_ "github.com/lib/pq"
	"github.com/pressly/goose"
	"go.uber.org/zap"
)

const head = "YaPrShortener"

// MigrationUpDB реализует миграцию Up БД. Возвращает ошибку.
//
// Параметры:
//
//	db - указатель на БД.
func MigrationUpDB(db *sql.DB) error {

	// Определение рабочей директории.
	path, err := workDir()
	if err != nil {
		return fmt.Errorf("ошибка при определении рабочей дирекории: <%w>", err)
	}

	// Определение пути к файлам миграции.
	var pathFilesMigration string

	switch path {
	case head + "/internal/service/db":
		pathFilesMigration = "../../../migrations"
	case head + "/cmd/shortener":
		pathFilesMigration = "../../migrations"
	case head:
		pathFilesMigration = "migrations"
	case head + "/" + head: // для тестов в github.
		pathFilesMigration = "migrations"
	default:
		return errors.New("не найдено совпадение пути в switch")
	}

	// Применение миграций.
	err = goose.Up(db, pathFilesMigration)
	if err != nil {
		logger.Log.Error("Ошибка применения миграции БД",
			zap.Error(err),
		)
		return fmt.Errorf("ошибка мри миграции БД")
	}

	return nil
}

// workDir определяет рабочую директорию. Возвращает директорию и ошибку.
func workDir() (string, error) {

	// Получения текущей дериктории проекта.
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("ошибка при определении рабочей директории: <%w>", err)
	}

	// Обработка данных директории.
	pathFull := strings.Split(dir, "/")
	startIndex := 0

	for i, v := range pathFull {
		if v == head {
			startIndex = i
			break
		}
	}

	if startIndex == 0 {
		return "", fmt.Errorf("голова проекта не найдена: <%s>", head)
	}

	// Формирование пути относительно головы проекта.
	path := strings.Join(pathFull[startIndex:], "/")

	return path, nil
}

// ConnectDB реализует подключение к БД. Возвращает указатель на БД, функцию отключения и ошибку.
//
// Параметры:
//
//	dsn - строка подключения к БД.
func ConnectDB(dsn string) (*sql.DB, func(), error) {

	// Проверка аргументов.
	if dsn == "" {
		return nil, nil, fmt.Errorf("в аргументе dsn нет содержмиого")
	}

	// Подключение к БД.
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Log.Error("Ошибка при подключении к БД", zap.Error(err))
		return nil, nil, fmt.Errorf("ошибка подключения к БД:<%v>", err)
	}

	// Ping.
	if err := db.Ping(); err != nil {
		logger.Log.Error("Ошибка Ping после подключения к БД", zap.Error(err))
		return nil, nil, fmt.Errorf("ошибка ping после подключения к БД:<%v>", err)
	}

	// Функция, для закрытия подключения к БД.
	closeDB := func() {
		if err := db.Close(); err != nil {
			logger.Log.Error("Ошибка при закрытии подключения к БД", zap.Error(err))
		}
	}

	// Результат.
	return db, closeDB, nil
}
