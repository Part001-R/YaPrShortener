package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// ShortURLFromLong пример использования.
func ExampleShortLong_ShortURLFromLong() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Создание HTTP сервера с использованием метода ShortURLFromLong.
	path := "/"
	mux := http.NewServeMux()
	mux.HandleFunc(path, shortener.ShortURLFromLong)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса
	dataBody := "https://practicum.yandex.ru/"
	req, err := http.NewRequest("POST", server.URL+path, bytes.NewBuffer([]byte(dataBody)))
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}
	req.Header.Set("Content-Type", "text/plain")

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	// Обработка ответа
	rxData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ошибка при чтении тела запроса: <%v>\n", err)
		return
	}
	fmt.Println(string(rxData))
}

// ShortURLFromLongBatch пример использования.
func ExampleShortLong_ShortURLFromLongBatch() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	path := "/api/shorten/batch"

	// Создание HTTP сервера с использованием метода ShortURLFromLongBatch.
	mux := http.NewServeMux()
	mux.HandleFunc(path, shortener.ShortURLFromLongBatch)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса
	type txLongURLBatch struct {
		CorrelationID string `json:"correlation_id"`
		OriginalURL   string `json:"original_url"`
	}
	txData := []txLongURLBatch{
		{
			CorrelationID: "123",
			OriginalURL:   "https://practicum.yandex.ru/",
		},
	}

	txByte, err := json.Marshal(txData)
	if err != nil {
		fmt.Printf("Ошибка сериализации данных: <%v>\n", err)
		return
	}

	req, err := http.NewRequest("POST", server.URL+path, bytes.NewBuffer([]byte(txByte)))
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	// Обработка ответа
	rxByte, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ошибка при чтении тела запроса: <%v>\n", err)
		return
	}

	type rxShortURLBatch struct {
		CorrelationID string `json:"correlation_id"`
		ShortURL      string `json:"short_url"`
	}
	rxData := make([]rxShortURLBatch, 0)

	err = json.Unmarshal(rxByte, &rxData)
	if err != nil {
		fmt.Printf("ошибка десериализации ответа: <%v>\n", err)
		return
	}

	fmt.Println(rxByte)
}

// LongURLFromShort пример использования.
func ExampleShortLong_LongURLFromShort() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Создание HTTP сервера с использованием метода ShortURLFromLong.
	mux := http.NewServeMux()
	mux.HandleFunc("/{id}", shortener.LongURLFromShort)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса
	// Значение foo, может быть другим и должно быть заранее известным.
	path := "/foo"
	req, err := http.NewRequest("GET", server.URL+path, nil)
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}
	req.Header.Set("Content-Type", "text/plain")

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	// Обработка ответа
	rxData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ошибка при чтении тела запроса: <%v>\n", err)
	}
	fmt.Println(string(rxData))
}

// ShortURLFromLongJSON пример использования.
func ExampleShortLong_ShortURLFromLongJSON() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Создание HTTP сервера с использованием метода ShortURLFromLongJSON.
	path := "/api/shorten"
	mux := http.NewServeMux()
	mux.HandleFunc(path, shortener.ShortURLFromLongJSON)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса
	type txLongURL struct {
		URL string `json:"url"`
	}
	txData := txLongURL{
		URL: "https://practicum.yandex.ru/",
	}
	txByte, err := json.Marshal(txData)
	if err != nil {
		fmt.Printf("ошибка сериализации данных: <%v>\n", err)
	}

	req, err := http.NewRequest("POST", server.URL+path, bytes.NewBuffer([]byte(txByte)))
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	// Обработка ответа
	rxByte, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ошибка при чтении тела запроса: <%v>\n", err)
		return
	}

	type rxShortURL struct {
		Result string `json:"result"`
	}
	var rxData rxShortURL

	err = json.Unmarshal(rxByte, &rxData)
	if err != nil {
		fmt.Printf("ошибка десериализации ответа: <%v>\n", err)
		return
	}

	fmt.Println(string(rxByte))
}

// PingDB пример использования.
func ExampleShortLong_PingDB() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Создание HTTP сервера с использованием метода PingDB.
	path := "/ping"
	mux := http.NewServeMux()
	mux.HandleFunc(path, shortener.PingDB)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса
	req, err := http.NewRequest("GET", server.URL+path, nil)
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("принят код: <%d>", resp.StatusCode)

	// При успешном выполнении запроса, возвращается http.StatusOK (200).
	// Иначе - http.StatusInternalServerError (500)
}

// UserURLs пример использования.
func ExampleShortLong_UserURLs() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Создание HTTP сервера с использованием метода UserURLs.
	path := "/api/user/urls"
	mux := http.NewServeMux()
	mux.HandleFunc(path, shortener.UserURLs)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса
	req, err := http.NewRequest("GET", server.URL+path, nil)
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	// Обработка ответа
	rxByte, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ошибка при чтении тела запроса: <%v>\n", err)
		return
	}

	type rxShortURLOriginalURL struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}

	rxData := make([]rxShortURLOriginalURL, 0)

	err = json.Unmarshal(rxByte, &rxData)
	if err != nil {
		fmt.Printf("ошибка десериализации: <%v>\n", err)
		return
	}

	fmt.Println(rxData)
}

// DeleteUserURLs пример использования
func ExampleShortLong_DeleteUserURLs() {

	// Подготовка конфигурации
	shortener := &ShortLong{
		List: &ShortLongURL{
			ShorByLong:  make(map[string]string),
			LongByShort: make(map[string]string),
			mu:          sync.RWMutex{},
		},
		DB:               &ShortLongDB{},
		BaseAddrShortURL: ":8080/",
		ServerAddr:       ":8080",
		FileStoragePath:  "storage.json",
	}

	// Создание HTTP сервера с использованием метода DeleteUserURLs.
	path := "/api/user/urls"
	mux := http.NewServeMux()
	mux.HandleFunc(path, shortener.DeleteUserURLs)
	server := httptest.NewServer(mux)

	// Создание HTTP клиента
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Подготовка запроса.
	// Значения сокращённых URL должны быть заранее известны.
	txData := []string{"foo", "bar"}
	txByte, err := json.Marshal(txData)
	if err != nil {
		fmt.Printf("ошибка сериализации данных: <%v>\n", err)
		return
	}

	req, err := http.NewRequest("DELETE", server.URL+path, bytes.NewBuffer([]byte(txByte)))
	if err != nil {
		fmt.Printf("ошибка подготовки запроса: <%v>\n", err)
		return
	}
	req.Header.Set("Authorization", "oups") // Данные аторизации должны быть известны.

	// Отправка запроса
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("ошибка запроса: <%v>\n", err)
		return
	}
	defer resp.Body.Close()

	// При успешной обработке запроса, возвращается http.StatusAccepted (202)
	fmt.Printf("код ответа: <%d>\n", resp.StatusCode)
}
