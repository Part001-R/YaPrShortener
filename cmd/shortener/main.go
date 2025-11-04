// main основной пакет приложения.
package main

import (
	"log"
	"runtime/debug"

	"github.com/Part001-R/YaPrShortener/internal/service"
)

func main() {

	// Перехват паники.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Паника в приложении: %v\n Стек вызовов:\n%s", r, string(debug.Stack()))
		}
	}()

	// Запуск приложения.
	if err := service.Run(); err != nil {
		log.Fatalf("Работа прервана по причине: {%v}", err)
	}
}
