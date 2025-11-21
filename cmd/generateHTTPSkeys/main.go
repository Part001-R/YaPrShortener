// main, содержит функционал по созданию сертификатов для HTTPS.
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {

	// Определение рабочей директории.
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("ошибка при определении рабочей директории: <%v>", err)
	}

	// Процесс создания ключей.
	fmt.Println("Генерация ключей для HTTPS")
	fmt.Println()

	// Публичный ключ
	var pubKeyName string
	fmt.Print("Введите название для публичного ключа и нажмите Enter: ")
	_, err = fmt.Scanln(&pubKeyName)
	if err != nil {
		log.Fatalf("ошибка чтения ввода имени публичного ключа: <%v>", err)
	}
	pubKeyName = pubKeyName + ".pem"

	// Приватный ключ
	var privKeyName string
	fmt.Print("Введите название для приватного ключа и нажмите Enter: ")
	fmt.Scanln(&privKeyName)
	if err != nil {
		log.Fatalf("ошибка чтения ввода имени приватного ключа: <%v>", err)
	}
	privKeyName = privKeyName + ".pem"

	// Номер сертификата
	var sertNumbStr string
	fmt.Print("Введине номер для сертификата и нажмите Enter: ")
	fmt.Scanln(&sertNumbStr)

	sertNumb, err := strconv.Atoi(sertNumbStr)
	if err != nil {
		log.Fatalf("ошибка при вводе номера сертифката: <%v>", err)
	}

	fmt.Println()

	// Результат ввода.
	fmt.Println("Для создания сертификатов, будут использоваться эти данные:")
	fmt.Printf("    Файл для публичного ключа: %s\n", pubKeyName)
	fmt.Printf("    Файл для приватного ключа: %s\n", privKeyName)
	fmt.Printf("    Номер сертификата: %d\n", sertNumb)
	fmt.Println()
	fmt.Printf("Файлы будут созданы в директории: <%s>\n", dir)
	fmt.Println()
	fmt.Print("Для создания файлов нажмите на Enter...")

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')

	// Создание
	GenerateKeys(dir, pubKeyName, privKeyName, int64(sertNumb))
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
