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

// Функция реализует миграцию Up БД. Возвращает ошибку.
//
// Параметры:
//
// config - конфигурация.
func MigrationUpDB(db *sql.DB) error {

	// Определение рабочей директории
	path, err := workDir()
	if err != nil {
		return fmt.Errorf("ошибка при определении рабочей дирекории: <%w>", err)
	}

	// Определение пути к файлам миграции
	var pathFilesMigration string

	switch path {
	case "YaPrShortener/internal/service/db":
		pathFilesMigration = "../../../migrations"
	case "YaPrShortener/cmd/shortener":
		pathFilesMigration = "../../migrations"
	case "YaPrShortener":
		pathFilesMigration = "migrations"
	case "YaPrShortener/YaPrShortener": // для тестов в github
		pathFilesMigration = "migrations"
	default:
		return errors.New("не найдено совпадение пути в switch")
	}

	// Применение миграций
	err = goose.Up(db, pathFilesMigration)
	if err != nil {
		logger.Log.Error("Ошибка применения миграции БД",
			zap.Error(err),
		)
		return fmt.Errorf("ошибка мри миграции БД")
	}

	return nil
}

// Функция определяет рабочую директорию. Возвращает директорию и ошибку.
func workDir() (string, error) {

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("ошибка при определении рабочей директории: <%w>", err)
	}

	pathFull := strings.Split(dir, "/")
	startIndex := 0
	headProject := "YaPrShortener"

	for i, v := range pathFull {
		if v == headProject {
			startIndex = i
			break
		}
	}

	if startIndex == 0 {
		return "", fmt.Errorf("голова проекта не найдена: <%s>", headProject)
	}

	// Формирование пути относительно головы проекта
	path := strings.Join(pathFull[startIndex:], "/")

	return path, nil
}

// Функция реализует подключение к БД. Возвращает указатель на БД, функцию отключения и ошибку.
//
// Параметры:
//
// dsn - строка подключения к БД.
func ConnectDB(dsn string) (*sql.DB, func(), error) {

	if dsn == "" {
		return nil, nil, fmt.Errorf("в аргументе dsn нет содержмиого")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Log.Error("Ошибка при подключении к БД",
			zap.Error(err),
		)
	}

	closeDB := func() {
		if err := db.Close(); err != nil {
			logger.Log.Error("Ошибка при закрытии подключения к БД",
				zap.Error(err),
			)
		}
	}

	return db, closeDB, nil
}
