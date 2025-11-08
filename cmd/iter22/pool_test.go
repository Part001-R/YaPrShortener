package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Pool_SUCCESS(t *testing.T) {

	// Конструктор
	examplePool := New[*ExampleType]()

	// Добавление объекта в пул.
	for i := 1; i <= 1; i++ {
		obj := &ExampleType{Value: i}
		examplePool.Put(obj)
	}

	// Получение объекта из пула
	obj := examplePool.Get()
	obj.Value = 111 // Присвоение значения объекту.

	// Возврат объекта в пул
	examplePool.Put(obj)

	// Проверка содержимого объекта.
	// Значение должны быть по умолчанию, т.к. есть сброс в Reset методе.
	obj = examplePool.Get()
	assert.Equalf(t, 0, obj.Value, "ожидался 0, а принято <%d>", obj.Value)
}
