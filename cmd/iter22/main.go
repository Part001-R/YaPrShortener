package main

import (
	"fmt"
)

func main() {
	// Конструктор
	examplePool := New[*ExampleType]()

	// Добавление объектов в пул.
	for i := 1; i <= 5; i++ {
		obj := &ExampleType{Value: i}
		examplePool.Put(obj)
	}

	// Получение объекта из пула
	obj := examplePool.Get()
	obj.Value = 111
	fmt.Printf("Содержимое объекта: <%d>\n", obj.Value)

	// Возврат объекта в пул
	examplePool.Put(obj)

	// Проверка содержимого объектов.
	// Значения должны быть по умолчанию, т.к. есть сброс в Reset методе.
	for i := 0; i < 5; i++ {
		obj := examplePool.Get()
		if obj != nil {
			fmt.Printf("Взято из пула: Value = %d\n", obj.Value)
		} else {
			fmt.Println("Взято из пула: nil")
		}
	}
}
