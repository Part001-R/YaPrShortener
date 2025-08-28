package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const defaultEndpoint = "http://localhost:8080/"

func main() {
	endpoint := getEndpoint()

	data := url.Values{}

	fmt.Println("Введите длинный URL")

	// открываем потоковое чтение из консоли
	reader := bufio.NewReader(os.Stdin)

	// читаем строку из консоли
	long, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	long = strings.TrimSuffix(long, "\n")

	// заполняем контейнер данными
	data.Set("url", long)

	// добавляем HTTP-клиент
	client := &http.Client{}

	// запрос
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		panic(err)
	}

	// в заголовках запроса указываем кодировку
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// отправляем запрос и получаем ответ
	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("ошибка при выполнении запроса: <%v>", err)
	}

	fmt.Println("Статус-код ", response.Status)
	defer func() {
		_ = response.Body.Close()
	}()

	// читаем поток из тела ответа
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("ошибка при чтении тела ответа: <%v>", err)
	}

	fmt.Println(string(body))
}

func getEndpoint() string {
	return defaultEndpoint
}
