// flags пакет для взаимодействия с флагами командной строки.
// Инициализируются переменные экземпляра при запуске приложения.
package flags

import (
	"flag"
	"os"
	"sync"
)

// Флаги сервиса.
// generate:reset
type Config struct {
	ServerAddr       string
	BaseAddrShortURL string
	LogLevel         string
	FileStoragePath  string
	AuditFile        string
	AuditURL         string
	DSNDB            string
}

// Обеспечение однократного выполнения.
var once sync.Once

// Данные флагов.
var flags = Config{}

// Реализация парсинга флагов. Возвращаются флаги.
func ParseFlags() Config {

	once.Do(func() {

		flag.StringVar(&flags.ServerAddr, "a", ":8080", "адрес и порт сервера")
		flag.StringVar(&flags.BaseAddrShortURL, "b", "http://localhost:8080/", "базовый адрес для коротких URL")
		flag.StringVar(&flags.LogLevel, "l", "info", "уровень логирования")
		flag.StringVar(&flags.FileStoragePath, "f", "storage.json", "хранилище ссылок")
		flag.StringVar(&flags.DSNDB, "d", "", "dsn подключения к БД")
		flag.StringVar(&flags.AuditFile, "audit-file", "", "путь к файлу-приёмнику")
		flag.StringVar(&flags.AuditURL, "audit-url", "", "URL удаленного сервера-приёмника")

		flag.Parse()

		if envValue := os.Getenv("SERVER_ADDRESS"); envValue != "" {
			flags.ServerAddr = envValue
		}
		if envValue := os.Getenv("BASE_URL"); envValue != "" {
			flags.BaseAddrShortURL = envValue
		}
		if envValue := os.Getenv("LOG_LEVEL"); envValue != "" {
			flags.LogLevel = envValue
		}
		if envValue := os.Getenv("FILE_STORAGE_PATH"); envValue != "" {
			flags.FileStoragePath = envValue
		}
		if envValue := os.Getenv("DATABASE_DSN"); envValue != "" {
			flags.DSNDB = envValue
		}
		if envValue := os.Getenv("AUDIT_FILE"); envValue != "" {
			flags.AuditFile = envValue
		}
		if envValue := os.Getenv("AUDIT_URL"); envValue != "" {
			flags.AuditURL = envValue
		}
	})

	return flags
}
