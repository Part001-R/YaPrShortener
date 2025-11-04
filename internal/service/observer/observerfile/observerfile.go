// observerfile пакет наблюдателя file. Секция методов.
//
// GetID - получение ID наблюдателя.
// SendMsg - передача оповещения.
package observerfile

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"go.uber.org/zap"
)

// Получение ID наблюдателя. Возвращается ID.
func (of *obsFile) GetID() string {
	return of.name
}

// SendMsg реализует сохранение сообщения в файл. Возвращается ошибка.
//
// Параметры:
//
//	msg - сообщение оповещения.
func (of *obsFile) SendMsg(msg observer.AuditEvent) error {

	of.mtx.Lock()
	defer of.mtx.Unlock()

	file, err := os.OpenFile(of.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		of.log.Error("Ошибка открытия файла аудита", zap.Error(err))
		return fmt.Errorf("ошибка открытия файла аудита: <%v>", err)
	}
	defer file.Close()

	msg.URL = strings.Trim(msg.URL, `\"`)

	data, err := json.Marshal(msg)
	if err != nil {
		of.log.Error("Ошибка json.Marshal", zap.Error(err))
		return fmt.Errorf("ошибка json.Marshal: <%v>", err)
	}
	file.WriteString(string(data) + "\n")
	return nil
}
