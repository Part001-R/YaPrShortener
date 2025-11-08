package main

import (
	"sync"
)

// Resettable интерфейс определяет метод Reset для типов, которые могут быть сброшены.
type Resettable interface {
	Reset()
}

// pool тип данных, представляющий хранилище объектов.
type pool[T Resettable] struct {
	mu  sync.Mutex
	obj []T
}

// Пул
var registeredPool interface{}

// Конструктор.
func New[T Resettable]() *pool[T] {

	if registeredPool == nil {
		registeredPool = &pool[T]{
			mu:  sync.Mutex{},
			obj: []T{},
		}
	}
	return registeredPool.(*pool[T])
}

// Get, извлекает объект из пула.
func (p *pool[T]) Get() T {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.obj) == 0 {
		var t T // Возврат нулевого значения типа T, если пул пуст.
		return t
	}

	// Извлечение объекта
	obj := p.obj[len(p.obj)-1]
	p.obj = p.obj[:len(p.obj)-1]
	return obj
}

// Put, возвращает объект в пул, сбрасывая его состояние.
//
// Параметры:
//
//	obj - объект.
func (p *pool[T]) Put(obj T) {
	p.mu.Lock()
	defer p.mu.Unlock()

	obj.Reset()
	p.obj = append(p.obj, obj)
}
