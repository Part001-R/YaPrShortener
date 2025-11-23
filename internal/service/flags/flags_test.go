// flags, тесты функций пакета.
package flags

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Тестирование функции ParseFlags. Используются env и флаги.
func Test_ParseFlags_SUCCESS(t *testing.T) {

	testData := []struct {
		testName        string
		argCmd          string
		argA            string
		argB            string
		argL            string
		argF            string
		argU            string
		argS            string
		wantAddr        string
		wantBase        string
		wantLogLevel    string
		wantFile        string
		wantEnableHTTPS string
	}{
		{
			testName:        "correct data",
			argCmd:          "cmd",
			argA:            "-a=localhost:9999",
			argB:            "-b=http://localhost:5500/",
			argL:            "-l=info",
			argF:            "-f=test.json",
			argU:            "http://localhost:1234/",
			argS:            "-s=true",
			wantAddr:        "localhost:9999",
			wantBase:        "http://localhost:5500/",
			wantLogLevel:    "info",
			wantFile:        "test.json",
			wantEnableHTTPS: "true",
		},
	}

	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			// Взаимодействие с аргументами.
			teardown := setup([]string{tt.argCmd, tt.argA, tt.argB, tt.argL, tt.argF, tt.argS})
			defer teardown()

			// Логика.
			flags := ParseFlags()
			assert.Equalf(t, tt.wantAddr, flags.Port, "ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.Port)
			assert.Equalf(t, tt.wantBase, flags.BaseAddrShortURL, "ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.BaseAddrShortURL)
			assert.Equalf(t, tt.wantLogLevel, flags.LogLevel, "ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.LogLevel)
			assert.Equalf(t, tt.wantFile, flags.FileStoragePath, "ожидалось {%s}, а принято {%s}", tt.wantFile, flags.FileStoragePath)
			assert.Equalf(t, tt.wantEnableHTTPS, flags.EnableHTTPS, "ожидалось {%s}, а принято {%s}", tt.wantEnableHTTPS, flags.EnableHTTPS)
		})
	}
}

// Тестирование функции ParseFlags. Установленные данные из фала заменяются данными из env или флагов.
func Test_ParseFlags_UseFile_Over_SUCCESS(t *testing.T) {

	testData := []struct {
		testName        string
		argCmd          string
		argA            string
		argB            string
		argL            string
		argF            string
		argU            string
		argS            string
		argC            string
		wantAddr        string
		wantBase        string
		wantLogLevel    string
		wantFile        string
		wantEnableHTTPS string
	}{
		{
			testName:        "Данные из файла перезаписываются",
			argCmd:          "cmd",
			argA:            "-a=localhost:9999",
			argB:            "-b=http://localhost:5500/",
			argL:            "-l=info",
			argF:            "-f=test.json",
			argU:            "http://localhost:1234/",
			argS:            "-s=true",
			argC:            "-c=config.json",
			wantAddr:        "localhost:9999",
			wantBase:        "http://localhost:5500/",
			wantLogLevel:    "info",
			wantFile:        "test.json",
			wantEnableHTTPS: "true",
		},
	}

	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			// Взаимодействие с аргументами.
			teardown := setup([]string{tt.argCmd, tt.argA, tt.argB, tt.argL, tt.argF, tt.argS, tt.argC})
			defer teardown()

			// Логика.
			flags := ParseFlags()
			assert.Equalf(t, tt.wantAddr, flags.Port, "В Port ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.Port)
			assert.Equalf(t, tt.wantBase, flags.BaseAddrShortURL, "В BaseAddrShortURL ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.BaseAddrShortURL)
			assert.Equalf(t, tt.wantLogLevel, flags.LogLevel, "В LogLevel ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.LogLevel)
			assert.Equalf(t, tt.wantFile, flags.FileStoragePath, "В FileStoragePath ожидалось {%s}, а принято {%s}", tt.wantFile, flags.FileStoragePath)
			assert.Equalf(t, tt.wantEnableHTTPS, flags.EnableHTTPS, "В EnableHTTPS ожидалось {%s}, а принято {%s}", tt.wantEnableHTTPS, flags.EnableHTTPS)
		})
	}
}

// Тестирование функции ParseFlags. Частичная установка значений из файла.
func Test_ParseFlags_UseFile_PartOver_SUCCESS(t *testing.T) {

	testData := []struct {
		testName        string
		argCmd          string
		argC            string
		wantAddr        string
		wantBase        string
		wantFile        string
		wantDSN         string
		wantEnableHTTPS string
	}{
		{
			testName:        "Частичная установка",
			argCmd:          "cmd",
			argC:            "-c=config.json",
			wantAddr:        ":8080",                  // по умолчанию - :8080.                       В файле - "localhost:8080"
			wantBase:        "http://localhost:8080/", // по умолчанию - http://localhost:8080/.      В файле - "http://localhost"
			wantFile:        "storage.json",           // по умолчанию - storage.json.                В файле - "storage.json"
			wantDSN:         "TestDSN",                // нет по умолчанию. Данные берутся из файла.  В файле - "TestDSN"
			wantEnableHTTPS: "false",                  // по умолчанию - false.                       В файле - true
		},
	}

	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			// Взаимодействие с аргументами.
			teardown := setup([]string{tt.argCmd, tt.argC})
			defer teardown()

			// Логика.
			flags := ParseFlags()
			assert.Equalf(t, tt.wantAddr, flags.Port, "В Port ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.Port)
			assert.Equalf(t, tt.wantBase, flags.BaseAddrShortURL, "В BaseAddrShortURL ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.BaseAddrShortURL)
			assert.Equalf(t, tt.wantFile, flags.FileStoragePath, "В FileStoragePath ожидалось {%s}, а принято {%s}", tt.wantFile, flags.FileStoragePath)
			assert.Equalf(t, tt.wantDSN, flags.DSNDB, "В DSNDB ожидалось {%s}, а принято {%s}", tt.wantDSN, flags.DSNDB)
			assert.Equalf(t, tt.wantEnableHTTPS, flags.EnableHTTPS, "В EnableHTTPS ожидалось {%s}, а принято {%s}", tt.wantEnableHTTPS, flags.EnableHTTPS)
		})
	}
}

// Тестирование функции readConfigFile.
func Test_readConfigFile_SUCCESS(t *testing.T) {

	nameFile := "config.json"

	conf, err := readConfigFile(nameFile)
	require.NoErrorf(t, err, "ошибка:<%v> при чтении файла конфигурации", err)

	assert.Equal(t, "localhost:8080", conf.Port, "у server_address, ожидалось <%s> а принято <%s>", "localhost:8080", conf.Port)
	assert.Equal(t, "http://localhost", conf.BaseAddrShortURL, "у file_storage_path, ожидалось <%s> а принято <%s>", "http://localhost", conf.BaseAddrShortURL)
	assert.Equalf(t, "storage.json", conf.FileStoragePath, "у storage.json, ожидалось <%s> а принято <%s>", "storage.json", conf.FileStoragePath)
	assert.Equalf(t, "TestDSN", conf.DSNDB, "у database_dsn, ожидалось <%s> а принято <%s>", "TestDSN", conf.DSNDB)
	assert.Equalf(t, true, conf.EnableHTTPS, "у enable_https, ожидалось <%t> а принято <%t>", true, conf.EnableHTTPS)
}

// setup, производит установку значений аргументов. Возвращает функцию с возвратом аргументов в исходное состояние.
func setup(osArgs []string) func() {
	originalArgs := os.Args
	os.Args = osArgs
	return func() {
		os.Args = originalArgs
	}
}
