package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Part001-R/YaPrShortener/internal/config/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// Функция реализует миграцию Up БД. Возвращает ошибку.
//
// Параметры:
//
// config - конфигурация.
func MigrationUpDB(config config.ConfigT) error {

	if config.DSNDB == "" {
		return nil
	}

	db, err := sql.Open("postgres", config.DSNDB)
	if err != nil {
		return fmt.Errorf("ошибка подключения к БД: <%w>", err)
	}
	defer db.Close()

	// Создание миграций
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("ошибка создания миграций: <%w>", err)
	}

	path, err := workDir()
	if err != nil {
		return fmt.Errorf("ошибка в распозновании пути проекта: <%w>", err)
	}

	// Определение места выполнения и формирование пути в migrations файлам
	var pathFilesMigration string

	switch path {
	case "YaPrShortener/internal/service/db":
		pathFilesMigration = "file://../../migrations"
	case "YaPrShortener/cmd/shortener":
		pathFilesMigration = "file://../migrations"
	case "YaPrShortener":
		pathFilesMigration = "file://migrations"
	default:
		return errors.New("не найдено совпадение пути в switch")
	}

	m, err := migrate.NewWithDatabaseInstance(
		pathFilesMigration,
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("ошибка работы с файлами: <%w>", err)
	}

	// Применение миграций
	if err := m.Up(); err != nil {
		if err.Error() == "no change" {
			return nil
		} else {
			return fmt.Errorf("ошибка выполнения миграции: <%w>", err)
		}
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
