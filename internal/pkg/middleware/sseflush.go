package middleware

import (
	"bufio"
	"net"
	"net/http"
	"strings"
)

// SSEFlush оборачивает http.ResponseWriter для SSE-роутов так, что после каждого
// Write автоматически вызывается Flusher.Flush(). Без этого http.Server держит
// чанки в bufio.Writer и клиент видит SSE-события только в финале стрима.
//
// Активируется только для путей, оканчивающихся на /messages с методом POST
// (POST /api/v1/chats/{id}/messages — единственный SSE-эндпоинт). Для остальных
// запросов passthrough без накладных расходов.
func SSEFlush(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isSSEPath(r) {
			next.ServeHTTP(w, r)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(&sseFlushWriter{ResponseWriter: w, flusher: flusher}, r)
	})
}

// isSSEPath распознаёт SSE-эндпоинт. Сейчас только POST /chats/{id}/messages.
func isSSEPath(r *http.Request) bool {
	return r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages")
}

// sseFlushWriter — обёртка ResponseWriter, которая флашит после каждого Write
type sseFlushWriter struct {
	http.ResponseWriter
	flusher http.Flusher
}

// Write пишет данные и сразу флашит, чтобы клиент видел SSE-события в реальном времени
func (w *sseFlushWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.flusher.Flush()
	return n, err
}

// Flush проксирует явный Flush, если кто-то его вызовет извне
func (w *sseFlushWriter) Flush() {
	w.flusher.Flush()
}

// Hijack пробрасывает Hijack, если базовый ResponseWriter поддерживает (на случай
// future-needs; стандартный net/http server поддерживает Hijacker)
func (w *sseFlushWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
