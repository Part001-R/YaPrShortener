// serthttps, пакет по взаимодействию с HTTPS сертификатами.
package serthttps

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReadKeys, функция реализует чтение сертфикатов. Возвразает публичный, приватный ключи и ошибку .
//
// Параметры:
//
//	dir - директория с расположением сертификатов.
//	namePublic - имя файла публичного сертификата.
//	namePrivate - имя файла приватного сертификата.
func ReadKeys(dir, namePublic, namePrivate string) (*x509.Certificate, *rsa.PrivateKey, error) {

	// Чтение сертификатов.
	certificateBytes, err := os.ReadFile(filepath.Join(dir, namePublic))
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка чтения публичного сертификата: <%w>", err)
	}

	privateKeyBytes, err := os.ReadFile(filepath.Join(dir, namePrivate))
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка чтения приватного сертификата: <%w>", err)
	}

	// Декодирование сертификатов
	certificatePemBlock, _ := pem.Decode(certificateBytes)
	if certificatePemBlock == nil {
		return nil, nil, fmt.Errorf("ошибка декодирования публичного сертификата: <%w>", err)
	}

	privateKeyPemBlock, _ := pem.Decode(privateKeyBytes)
	if privateKeyPemBlock == nil {
		return nil, nil, fmt.Errorf("ошибка декодирования приватного сертификата: <%w>", err)
	}

	// Парсинг сертификатов
	certificate, err := x509.ParseCertificate(certificatePemBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка парсинга публичного сертификата: <%w>", err)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyPemBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка парсинга приватного сертификата: <%w>", err)
	}

	// Результат
	return certificate, privateKey, nil
}

// GenerateKeys, функция генерации публичного и приватного сертификатов. Возвращается ошибка.
//
// Параметры:
//
//	dir - директория с расположением сертификатов.
//	namePublic - имя файла публичного сертификата.
//	namePrivate - имя файла приватного сертификата.
//	numbSert - номер сертификатов.
func GenerateKeys(dir, namePublic, namePrivate string, numbSert int64) error {

	// создаём шаблон сертификата
	cert := &x509.Certificate{
		// указываем уникальный номер сертификата
		SerialNumber: big.NewInt(numbSert),
		// заполняем базовую информацию о владельце сертификата
		Subject: pkix.Name{
			Organization: []string{"Yandex.Praktikum"},
			Country:      []string{"RU"},
		},
		// разрешаем использование сертификата для 127.0.0.1 и ::1
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		// сертификат верен, начиная со времени создания
		NotBefore: time.Now(),
		// время жизни сертификата — 10 лет
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		// устанавливаем использование ключа для цифровой подписи,
		// а также клиентской и серверной авторизации
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// создаём новый приватный RSA-ключ длиной 4096 бит
	// обратите внимание, что для генерации ключа и сертификата
	// используется rand.Reader в качестве источника случайных данных
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("ошибка генерации приватного ключа: <%w>", err)
	}

	// создаём сертификат x.509
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("ошибка генерации публичного ключа: <%w>", err)
	}

	// кодируем сертификат и ключ в формате PEM, который
	// используется для хранения и обмена криптографическими ключами
	var certPEM bytes.Buffer
	err = pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		return fmt.Errorf("ошибка при кодироании CERTIFICATE: <%w>", err)
	}

	var privateKeyPEM bytes.Buffer
	err = pem.Encode(&privateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err != nil {
		return fmt.Errorf("ошибка при кодироании RSA PRIVATE KEY: <%w>", err)
	}

	// Сохранение сертификатов
	if err = os.WriteFile(filepath.Join(dir, namePublic), certPEM.Bytes(), 0644); err != nil {
		return fmt.Errorf("ошибка при сохранении публичного сертификата: <%w>", err)
	}

	if err = os.WriteFile(filepath.Join(dir, namePrivate), privateKeyPEM.Bytes(), 0644); err != nil {
		return fmt.Errorf("ошибка при сохранении приватного сертификата: <%w>", err)
	}

	return nil
}

// CheckExistFiles, функция проверяет существование файлов. Возвращает true - если файлы существуют и ошибку.
//
// Параметры:
//
//	dir - директория с расположением сертификатов.
//	namePublic - имя файла публичного сертификата.
//	namePrivate - имя файла приватного сертификата.
func CheckExistFiles(dir, namePublic, namePrivate string) (bool, error) {

	// Проверка существования публичного ключа
	var strPub strings.Builder

	strPub.WriteString(dir)
	strPub.WriteString("/")
	strPub.WriteString(namePublic)
	pub := strPub.String()

	_, err := os.Stat(pub)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ошибка:<%w> при проверку существования файла: <%s>", err, namePublic)
	}

	// Проверка существования приватного ключа
	var strPriv strings.Builder

	strPriv.WriteString(dir)
	strPriv.WriteString("/")
	strPriv.WriteString(namePrivate)
	priv := strPriv.String()

	_, err = os.Stat(priv)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("ошибка:<%w> при проверку существования файла: <%s>", err, namePrivate)
	}

	// Результат
	return true, nil
}
