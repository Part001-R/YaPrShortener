package authorisation

import (
	"net/http"

	"github.com/google/uuid"
)

type User struct {
	ID string
}

// Функцуия выполняет проверку наличая куки с идентификатором пользователя и устанавливать её, если она отсутствует или недействительна.
//
// Параметры:
//
// w - http.ResponseWriter.
// userID - ID пользователя.
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

// Функция выполняет генерацию уникального ID. Возвращает уникальный ID.
func GenerateUniqueID() string {
	return uuid.New().String()
}
