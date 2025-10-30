// gz пакет для работы с архивами gzip.
package gz

import (
	"compress/gzip"
	"io"
	"net/http"
)

// compressWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера.
// Сжимать передаваемые данные и выставлять правильные HTTP-заголовки.
type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

// NewCompressWriter конструктор. Возвращает экземпляр writer.
//
// Параметры:
//
//	w - интерфейс ответа.
func NewCompressWriter(w http.ResponseWriter) *compressWriter {

	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// Header обрабатывает заголовки. Возвращаются заголовки.
func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

// Write реализует запись. Возвращается количество записанных байт и ошибка.
//
// Параметры:
//
//	p - байты для записи.
func (c *compressWriter) Write(p []byte) (int, error) {
	return c.zw.Write(p)
}

// WriteHeader добавляет заголовок Content-Encoding.
//
// Параметры:
//
//	statusCode - код статуса.
func (c *compressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 {
		c.w.Header().Set("Content-Encoding", "gzip")
	}
	c.w.WriteHeader(statusCode)
}

// Close закрывает gzip.Writer и досылает все данные из буфера. Возвращается ошибка.
func (c *compressWriter) Close() error {
	return c.zw.Close()
}

// compressReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера
// декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

// --------------------------------------------------

// NewCompressReader конструктор. Возвращает экземпляр reader и ошибку.
//
// Параметры:
//
//	r - интерфейс чтения.
func NewCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

// Read реализует чтение. Возвращается количество прочитанных байт и ошибка.
//
// Параметры:
//
//	p - байты для  записи.
func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

// Close закрывает читателя. Возвращается ошибка.
//
// Параметры:
//
//	c - указатель на reader.
func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}
