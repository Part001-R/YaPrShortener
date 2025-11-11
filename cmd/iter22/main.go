package main

import (
	"fmt"
)

func main() {
	// Конструктор
	pool := New[*ExampleType]()

	// Добавление объектов в пул.
	for i := 1; i <= 5; i++ {
		obj := &ExampleType{Value: i}
		pool.Put(obj)
	}

	// Получение объекта из пула
	obj := pool.Get()
	obj.Value = 111
	fmt.Printf("Содержимое объекта: <%d>\n", obj.Value)

	// Возврат объекта в пул
	pool.Put(obj)

	// Проверка содержимого объектов.
	// Значения должны быть по умолчанию, т.к. есть сброс в Reset методе.
	for i := 0; i < 5; i++ {
		obj := pool.Get()
		if obj != nil {
			fmt.Printf("Взято из пула: Value = %d\n", obj.Value)
		} else {
			fmt.Println("Взято из пула: nil")
		}
	}
}
