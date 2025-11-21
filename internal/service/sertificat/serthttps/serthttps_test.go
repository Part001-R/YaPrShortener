// serthttps, тесты.
package serthttps

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_Keys_SUCCESS, тест проверки генерации ключей, шифровки и расшифровки.
func Test_Keys_SUCCESS(t *testing.T) {

	dir, err := os.Getwd()
	require.NoErrorf(t, err, "ошибка в определении директории: <%w>", err)

	// Создание сертификатов.
	keyPublic := "public.pem"
	keyPrivate := "private.pem"
	err = GenerateKeys(dir, keyPublic, keyPrivate, 123)
	require.NoErrorf(t, err, "ошибка при создании сертификатов: <%v>", err)

	// Чтение сертификатов.
	public, private, err := ReadKeys(dir, keyPublic, keyPrivate)
	require.NoErrorf(t, err, "ошибка чтения сертификатов: <%v>", err)

	message := []byte("message") // сообщение

	// Шифрование публичным ключом.
	encryptedMessage, err := rsa.EncryptPKCS1v15(rand.Reader, public.PublicKey.(*rsa.PublicKey), message)
	require.NoErrorf(t, err, "ошибка при шифровании сообщения: <%v>", err)

	// Расшифровка приватным ключом.
	decryptedMessage, err := rsa.DecryptPKCS1v15(rand.Reader, private, encryptedMessage)
	require.NoErrorf(t, err, "ошибка при расшифровки сообщения: <%v>", err)

	// Проверка на соответствие.
	assert.Equalf(t, message, decryptedMessage, "ожидалось:<%s>, а принято:<%s>", message, decryptedMessage)

	// Удаление файлов.
	err = os.Remove(keyPublic)
	assert.NoErrorf(t, err, "ошибка при удалении публичного ключа: <%v>", err)

	err = os.Remove(keyPrivate)
	assert.NoErrorf(t, err, "ошибка при удалении приватного ключа: <%v>", err)
}

// Test_CheckExistFiles, проверка существования файлов.
func Test_CheckExistFiles(t *testing.T) {

	keyPublic := "public.pem"
	keyPrivate := "private.pem"

	dir, err := os.Getwd()
	require.NoErrorf(t, err, "ошибка в определении директории: <%w>", err)

	// Проверка отработки при отсутствии файлов.
	ok, err := CheckExistFiles(dir, keyPublic, keyPrivate)
	require.NoErrorf(t, err, "ошибка при проверке существования файлой до создания: <%v>", err)
	require.Equalf(t, false, ok, "ожидался false, а принято: <%t>", ok)

	// Создание сертификатов.
	err = GenerateKeys(dir, keyPublic, keyPrivate, 123)
	require.NoErrorf(t, err, "ошибка при создании сертификатов: <%v>", err)

	// Проверка отработки при существующих файлах.
	ok, err = CheckExistFiles(dir, keyPublic, keyPrivate)
	require.NoErrorf(t, err, "ошибка при проверке существования файлой до создания: <%v>", err)
	require.Equalf(t, true, ok, "ожидался true, а принято: <%t>", ok)

	// Удаление файлов.
	err = os.Remove(keyPublic)
	assert.NoErrorf(t, err, "ошибка при удалении публичного ключа: <%v>", err)

	err = os.Remove(keyPrivate)
	assert.NoErrorf(t, err, "ошибка при удалении приватного ключа: <%v>", err)
}
