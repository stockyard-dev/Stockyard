package engine

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
)

// recoveryMiddleware catches panics in any handler and returns a clean 500.
// Without this, Go's net/http recovery just drops the connection with no response.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("PANIC: %s %s — %v\n%s", r.Method, r.URL.Path, err, stack)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"error":{"message":"internal server error","type":"server_error"}}`)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
