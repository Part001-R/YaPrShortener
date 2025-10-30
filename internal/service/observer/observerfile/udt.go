// observerfile пакет наблюдателя file. Секция типов данных.
// Содержит конструктор.
package observerfile

import (
	"sync"

	"github.com/Part001-R/YaPrShortener/internal/service/observer"
)

// Представление наблюдателя.
type obsFile struct {
	name     string
	filePath string
}

// экземпляр наблюдателя.
var obs *obsFile

// Обеспечение единоразовой инициализации.
var once sync.Once

// NewObserverFile конструктор. Возвращается интерфейс наблюдателя.
//
// Параметры:
//
//	obsID - ID наблюдателя.
//	filePath - путь к файлу.
func NewObserverFile(obsID, filePath string) observer.ActionsObservers {
	once.Do(func() {
		obs = &obsFile{
			name:     obsID,
			filePath: filePath,
		}
	})

	return obs
}
