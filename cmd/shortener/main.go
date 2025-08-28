package main

import (
	"log"

	"github.com/Part001-R/YaPrShortener/internal/service"
)

func main() {
	if err := service.Run(); err != nil {
		log.Fatalf("работа прервана по причине: {%v}", err)
	}
}
