package flags

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

			os.Args = []string{tt.argCmd, tt.argA, tt.argB, tt.argL, tt.argF, tt.argS}

			flags := ParseFlags()
			assert.Equalf(t, tt.wantAddr, flags.Port, "ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.Port)
			assert.Equalf(t, tt.wantBase, flags.BaseAddrShortURL, "ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.BaseAddrShortURL)
			assert.Equalf(t, tt.wantLogLevel, flags.LogLevel, "ожидалось {%s}, а принято {%s}", tt.wantAddr, flags.LogLevel)
			assert.Equalf(t, tt.wantFile, flags.FileStoragePath, "ожидалось {%s}, а принято {%s}", tt.wantFile, flags.FileStoragePath)
			assert.Equalf(t, tt.wantEnableHTTPS, flags.EnableHTTPS, "ожидалось {%s}, а принято {%s}", tt.wantEnableHTTPS, flags.EnableHTTPS)
		})
	}
}
