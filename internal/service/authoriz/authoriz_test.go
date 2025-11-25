// authoriz, тесты.
package authoriz

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Проверка генерации UUID.
func Test_GenerateUniqueID_SUCCESS(t *testing.T) {

	val1 := GenerateUniqueID()
	val2 := GenerateUniqueID()

	require.NotEqualf(t, val1, val2, "значения <%s> и <%s>, одинаковые", val1, val2)
}

// Проверка Cookie.
func TestSetUserCookie(t *testing.T) {

	// Регистратор
	recorder := httptest.NewRecorder()
	userID := "test_user"

	// Вызов функции etUserCookie
	SetUserCookie(recorder, userID)

	// Получение cookie из ответа
	cookies := recorder.Result().Cookies()
	lenC := len(cookies)
	require.NotEqual(t, 0, lenC, "длинна куки = 0")

	cookie := cookies[0]

	// Проверка
	assert.Equal(t, "user_id", cookie.Name, "cookie.name нет соответствия")
	assert.Equal(t, userID, cookie.Value, "cookie.value нет соответствия")
	assert.Equal(t, "/", cookie.Path, "cookie.path нет соответствия")
	assert.True(t, cookie.HttpOnly, "expected.HttpOnly в false")
	assert.True(t, cookie.Secure, "expected.Secure в false")
}
