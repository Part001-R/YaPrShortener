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
	mu   sync.Mutex
	pool []T
}

// Пул
var registeredPool interface{}

// Конструктор.
func New[T Resettable]() *pool[T] {

	if registeredPool == nil {
		registeredPool = &pool[T]{
			mu:   sync.Mutex{},
			pool: []T{},
		}
	}
	return registeredPool.(*pool[T])
}

// Get, извлекает объект из пула.
func (p *pool[T]) Get() T {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pool) == 0 {
		var t T // Возврат нулевого значения типа T, если пул пуст.
		return t
	}

	// Извлечение объекта
	obj := p.pool[len(p.pool)-1]
	p.pool = p.pool[:len(p.pool)-1]
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
	p.pool = append(p.pool, obj)
}
