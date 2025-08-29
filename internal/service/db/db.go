package db

import (
	"database/sql"
	"fmt"

	"github.com/Part001-R/YaPrShortener/internal/config/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

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

	m, err := migrate.NewWithDatabaseInstance(
		"file://../../migrations",
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
