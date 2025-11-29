package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Keys_SUCCESS(t *testing.T) {

	dir, err := os.Getwd()
	require.NoErrorf(t, err, "ошибка в определении директории: <%w>", err)

	// Создание сертификатов.
	keyPublic := "public.pem"
	keyPrivate := "private.pem"
	err = GenerateKeys(dir, keyPublic, keyPrivate, 123)
	require.NoErrorf(t, err, "ошибка при создании сертификатов: <%v>", err)

	// Проверка существования файлов
	_, err = os.Stat(keyPublic)
	require.NoErrorf(t, err, "ошибка:<%v> при проверке существования файла:<%s>", err, keyPublic)

	_, err = os.Stat(keyPrivate)
	require.NoErrorf(t, err, "ошибка:<%v> при проверке существования файла:<%s>", err, keyPrivate)

	// Удаление файлов
	err = os.Remove(keyPublic)
	assert.NoErrorf(t, err, "ошибка при удалении публичного ключа: <%v>", err)

	err = os.Remove(keyPrivate)
	assert.NoErrorf(t, err, "ошибка при удалении приватного ключа: <%v>", err)
}
