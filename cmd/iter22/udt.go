// main, содержит типы данных и методы.
package main

// ExampleType, тип демонстрационного объекта.
type ExampleType struct {
	Value int
}

// Reset, реализует сброс содержимого объекта на значение по умолчанию.
func (e *ExampleType) Reset() {
	e.Value = 0
}
