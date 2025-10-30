// observerurl пакет наблюдателя URL. Секция с типами данных и конструктором.
// Содержит конструктор экземпляра.
package observerurl

import (
	"net/http"
	"sync"
	"time"

	"github.com/Part001-R/YaPrShortener/internal/service/observer"
)

// Представление наблюдателя.
type obsURL struct {
	name       string
	pathURL    string
	clientHTTP *http.Client
}

// Экземпляр наблюдателя.
var obs *obsURL

// Обеспечение единоразовой инициализации.
var once sync.Once

// NewObserverURL конструктор. Возвращается интерфейс.
//
// Параметры:
//
//	obsID - ID наблюдателя.
//	obsPath - URL наблюдателя.
func NewObserverURL(obsID, obsPath string) observer.ActionsObservers {
	once.Do(func() {

		client := &http.Client{
			Timeout: 2 * time.Second,
		}

		obs = &obsURL{
			name:       obsID,
			pathURL:    obsPath,
			clientHTTP: client,
		}
	})

	return obs
}
