// flags пакет для взаимодействия с флагами командной строки.
// Инициализируются переменные экземпляра при запуске приложения.
package flags

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Флаги сервиса.
// generate:reset
type Config struct {
	Port             string
	BaseAddrShortURL string
	LogLevel         string
	FileStoragePath  string
	AuditFile        string
	AuditURL         string
	DSNDB            string
	EnableHTTPS      string
	ConfigFile       string
}

// Для чтения конфигурационного файла.
type readData struct {
	Port             string `json:"server_address"`
	BaseAddrShortURL string `json:"base_url"`
	FileStoragePath  string `json:"file_storage_path"`
	DSNDB            string `json:"database_dsn"`
	EnableHTTPS      bool   `json:"enable_https"`
}

// Обеспечение однократного выполнения.
var once sync.Once

// Данные флагов.
var flags = Config{}

// Реализация парсинга флагов. Возвращаются флаги.
func ParseFlags() *Config {

	once.Do(func() {

		flag.StringVar(&flags.Port, "a", ":8080", "адрес и порт сервера")
		flag.StringVar(&flags.BaseAddrShortURL, "b", "http://localhost:8080/", "базовый адрес для коротких URL")
		flag.StringVar(&flags.LogLevel, "l", "info", "уровень логирования")
		flag.StringVar(&flags.FileStoragePath, "f", "storage.json", "хранилище ссылок")
		flag.StringVar(&flags.DSNDB, "d", "", "dsn подключения к БД")
		flag.StringVar(&flags.AuditFile, "audit-file", "", "путь к файлу-приёмнику")
		flag.StringVar(&flags.AuditURL, "audit-url", "", "URL удаленного сервера-приёмника")
		flag.StringVar(&flags.EnableHTTPS, "s", "false", "разрешение на запуск HTTPS")
		flag.StringVar(&flags.ConfigFile, "c", "", "путь к файлу конфигурации в формате JSON")

		flag.Parse()

		// Установка значений из фaйла.
		if err := setFromFile(&flags); err != nil {
			log.Fatalf("ошибка в ParseFlags. Функция setFromFile, вернула ошибку:<%v>", err)
		}

		// Установка значений из флагов и переменных окружения.
		if err := setFromFlagsEnv(&flags); err != nil {
			log.Fatalf("ошибка в ParseFlags. Функция setFromFlagsEnv, вернула ошибку:<%v>", err)
		}
	})

	return &flags
}

// readConfigFile, функция выполняет чтение конфигурационного файла. Возвращает json и ошибку.
//
// Параметры:
//
//	name - имя файла.
func readConfigFile(name string) (rd readData, err error) {

	// Проверка аргументов
	if name == "" {
		return readData{}, errors.New("в аргументе name, нет содержимого")
	}

	// Открытие файла
	f, err := os.Open(name)
	if err != nil {
		return readData{}, fmt.Errorf("ошибка:<%w>, при открытии файла:<%s>", err, name)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("ошибка при закрытии файла:<%w>", closeErr)
			}
		}
	}()

	// Чтение файла
	// Чтение всего файла
	data, err := io.ReadAll(f)
	if err != nil {
		return readData{}, fmt.Errorf("ошибка:<%w>, при чтении файла:<%s>", err, name)
	}

	// Десериализация данных
	if err := json.Unmarshal(data, &rd); err != nil {
		return readData{}, fmt.Errorf("ошибка:<%w>, десериализации данных файла:<%s>", err, name)
	}
	// Результат
	return rd, nil
}

// setFromFile, функция выполняет установку значений исходя из содержимого файла. Возвращает ошибку.
//
// Парамметры:
//
//	f - указатель на структуру.
func setFromFile(f *Config) error {

	// Проверка
	if f == nil {
		return errors.New("нет указателя в f")
	}

	// Логика
	//
	if envValue := os.Getenv("CONFIG"); envValue != "" {
		f.ConfigFile = envValue
	}

	// Чтение конфигурационного файла.
	var dataFromFile readData // Данные из конфигурационного файла.
	var err error
	if f.ConfigFile != "" {
		dataFromFile, err = readConfigFile(f.ConfigFile)
		if err != nil {
			return fmt.Errorf("функция readConfigFile, вернула ошибку:<%w>", err)
		}

		// Установка значений из файла
		if f.Port == "" {
			f.Port = dataFromFile.Port
		}
		if f.BaseAddrShortURL == "" {
			f.BaseAddrShortURL = dataFromFile.BaseAddrShortURL
		}
		if f.FileStoragePath == "" {
			f.FileStoragePath = dataFromFile.FileStoragePath
		}
		if f.DSNDB == "" {
			f.DSNDB = dataFromFile.DSNDB
		}
		if f.EnableHTTPS == "" {
			if dataFromFile.EnableHTTPS {
				f.EnableHTTPS = "true"
			} else {
				f.EnableHTTPS = "false"
			}
		}
	}

	return nil
}

// setFromFlagsEnv, функция выполняет установку значений исходя из содержимого флагов и env. Возвращает ошибку.
//
// Парамметры:
//
//	f - указатель на структуру.
func setFromFlagsEnv(f *Config) error {

	// Проверка
	if f == nil {
		return errors.New("нет указателя в f")
	}

	// Логика
	if envValue := os.Getenv("SERVER_ADDRESS"); envValue != "" {
		f.Port = envValue
	}
	if envValue := os.Getenv("BASE_URL"); envValue != "" {
		f.BaseAddrShortURL = envValue
	}
	if envValue := os.Getenv("LOG_LEVEL"); envValue != "" {
		f.LogLevel = envValue
	}
	if envValue := os.Getenv("FILE_STORAGE_PATH"); envValue != "" {
		f.FileStoragePath = envValue
	}
	if envValue := os.Getenv("DATABASE_DSN"); envValue != "" {
		f.DSNDB = envValue
	}
	if envValue := os.Getenv("AUDIT_FILE"); envValue != "" {
		f.AuditFile = envValue
	}
	if envValue := os.Getenv("AUDIT_URL"); envValue != "" {
		f.AuditURL = envValue
	}
	if envValue := os.Getenv("ENABLE_HTTPS"); envValue != "" {
		f.EnableHTTPS = envValue
	}

	return nil
}
