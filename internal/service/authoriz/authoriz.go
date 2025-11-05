// authoriz пакет, для установки cookie.
package authoriz

import (
	"net/http"

	"github.com/google/uuid"
)

// Представление пользователя.
type User struct {
	ID string
}

// Для использования в cookie
var UUID string

// SetUserCookie выполняет проверку наличая куки с идентификатором пользователя и устанавливать её, если она отсутствует или недействительна.
//
// Параметры:
//
//	w - http.ResponseWriter.
//	userID - ID пользователя.
func SetUserCookie(w http.ResponseWriter, userID string) {
	cookie := http.Cookie{
		Name:     "user_id",
		Value:    userID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}
	http.SetCookie(w, &cookie)
}

// GenerateUniqueID выполняет генерацию уникального ID. Возвращает уникальный ID.
func GenerateUniqueID() string {
	return uuid.New().String()
}
