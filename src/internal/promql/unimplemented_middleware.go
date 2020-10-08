package promql

import (
	"net/http"
	"strings"
)

func UnimplementedMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/query") {
			w.WriteHeader(http.StatusNotImplemented)
			w.Write([]byte(`{
				"status": "error",
				"errorType": "bad_data",
				"error": "Metrics not available"
			}`))
			return
		}
		h.ServeHTTP(w, r)
	})
}
