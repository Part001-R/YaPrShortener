package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Pool_SUCCESS(t *testing.T) {

	// Конструктор.
	pool := New[*ExampleType]()

	// Добавление объекта в пул.
	for i := 1; i <= 1; i++ {
		obj := &ExampleType{Value: i}
		pool.Put(obj)
	}

	// Получение объекта из пула.
	obj := pool.Get()
	obj.Value = 111 // Присвоение значния, для отслеживания сброса.

	// Возврат объекта в пул.
	pool.Put(obj)

	// Проверка содержимого объекта.
	// Значение должны быть по умолчанию, т.к. есть сброс в Reset методе.
	want := 0
	obj = pool.Get()
	assert.Equalf(t, want, obj.Value, "ожидался <%d>, а принято <%d>", want, obj.Value)
}
