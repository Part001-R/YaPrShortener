package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetValueOrDefault_SUCCESS(t *testing.T) {

	// Есть передаваемой значение.
	valueTx := "AAA"
	valueRx := GetValueOrDefault(valueTx)
	assert.Equalf(t, valueTx, valueRx, "ожидалось <%s>, а принято <%s>", valueTx, valueRx)

	// Нет передаваемого значения.
	want := "N/A"
	valueTx = ""
	valueRx = GetValueOrDefault(valueTx)
	assert.Equalf(t, want, valueRx, "ожидалось <%s>, а принято <%s>", want, valueRx)
}
