package config

import (
	"flag"
	"os"
)

type ConfigT struct {
	ServerAddr          string
	BaseAddrShortURL    string
	LogLevel            string
	FileStoragePath     string
	StoreIntervalMetr   string // Metrics
	FileStoragePathMetr string // Metrics
	RestoreMetr         string // Metrics
	DSNDB               string // Metrics
}

func ParseFlags() ConfigT {

	var flags = ConfigT{}

	// URL
	flag.StringVar(&flags.ServerAddr, "a", ":8080", "адрес и порт сервера")
	flag.StringVar(&flags.BaseAddrShortURL, "b", "http://localhost:8080/", "базовый адрес для коротких URL")
	flag.StringVar(&flags.LogLevel, "l", "info", "уровень логирования")
	flag.StringVar(&flags.FileStoragePath, "f", "storage.json", "хранилище ссылок")
	flag.StringVar(&flags.DSNDB, "d", "", "dsn подключения к БД")
	// Metrics
	flag.StringVar(&flags.StoreIntervalMetr, "i", "300", "периодичность сохранения метрик в файл")
	flag.StringVar(&flags.FileStoragePathMetr, "fm", "storageMetrics.json", "хранилище метрик")
	flag.StringVar(&flags.RestoreMetr, "r", "false", "загрузка данных из файла при старте")

	flag.Parse()

	// URL
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
	// Metrics
	if envValue := os.Getenv("STORE_INTERVAL_M"); envValue != "" {
		flags.StoreIntervalMetr = envValue
	}
	if envValue := os.Getenv("FILE_STORAGE_PATH_M"); envValue != "" {
		flags.FileStoragePathMetr = envValue
	}
	if envValue := os.Getenv("RESTORE_M"); envValue != "" {
		flags.RestoreMetr = envValue
	}

	return flags
}
