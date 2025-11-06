package tests

import (
	"log"
	"os"
)

func DoLogFatal() {
	log.Fatal("...") // want "найдено использование log.Fatal"
}

func DoExit() {
	os.Exit(1) // want "найдено использование os.Exit"
}

func DoPanic() {
	panic("...") // want "найдено использование panic"
}
