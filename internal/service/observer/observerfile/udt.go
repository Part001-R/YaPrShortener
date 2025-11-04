// observerfile пакет наблюдателя file. Секция типов данных.
// Содержит конструктор.
package observerfile

import (
	"sync"

	"github.com/Part001-R/YaPrShortener/internal/service/observer"
	"go.uber.org/zap"
)

// Представление наблюдателя.
type obsFile struct {
	name     string
	filePath string
	mtx      sync.Mutex
	log      *zap.Logger
}

// экземпляр наблюдателя.
var obs *obsFile

// Обеспечение единоразовой инициализации.
var once sync.Once

// NewObserverFile конструктор. Возвращается интерфейс наблюдателя.
//
// Параметры:
//
//		obsID - ID наблюдателя.
//		filePath - путь к файлу.
//	 log - логгер.
func NewObserverFile(obsID, filePath string, log *zap.Logger) observer.ActionsObservers {
	once.Do(func() {
		obs = &obsFile{
			name:     obsID,
			filePath: filePath,
			mtx:      sync.Mutex{},
			log:      log,
		}
	})

	return obs
}
