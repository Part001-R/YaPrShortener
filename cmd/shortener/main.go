// main основной пакет приложения.
package main

import (
	"log"
	"runtime/debug"

	"github.com/Part001-R/YaPrShortener/internal/service"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {

	// Перехват паники.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Паника в приложении: %v\n Стек вызовов:\n%s", r, string(debug.Stack()))
		}
	}()

	// Вывод информации о сборке.
	//
	// Пример использования:
	// go run -ldflags "-X main.buildVersion=1.0.0 -X main.buildDate=$(date +%Y-%m-%d) -X main.buildCommit=$(git rev-parse HEAD)" main.go
	// go build -ldflags "-X main.buildVersion=1.0.0 -X main.buildDate=$(date +%Y-%m-%d) -X main.buildCommit=$(git rev-parse HEAD)" -o myapp
	log.Printf("Build version: %s", service.GetValueOrDefault(buildVersion))
	log.Printf("Build date: %s", service.GetValueOrDefault(buildDate))
	log.Printf("Build commit: %s", service.GetValueOrDefault(buildCommit))

	// Запуск приложения.
	if err := service.Run(); err != nil {
		log.Fatalf("Работа прервана по причине: {%v}", err)
	}
}
