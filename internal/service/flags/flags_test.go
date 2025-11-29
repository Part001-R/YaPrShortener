// flags, тесты функций пакета.
package flags

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Тестирование функции ParseFlags.
func Test_ParseFlags_SUCCESS(t *testing.T) {

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
			testName:        "С использованием файла",
			argCmd:          "cmd",
			argC:            "-c=config.json",
			wantAddr:        ":8080",                  // по умолчанию - :8080.                       В файле - "localhost:8080"
			wantBase:        "http://localhost:8080/", // по умолчанию - http://localhost:8080/.      В файле - "http://localhost"
			wantFile:        "storage.json",           // по умолчанию - storage.json.                В файле - "storage.json"
			wantDSN:         "TestDSN",                // нет по умолчанию. Данные берутся из файла.  В файле - "TestDSN"
			wantEnableHTTPS: "false",                  // по умолчанию - false.                       В файле - true
		},
	}

	// Тесты.
	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			os.Args = []string{tt.argCmd, tt.argC}

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
