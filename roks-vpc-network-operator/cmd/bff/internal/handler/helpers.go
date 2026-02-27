package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs every HTTP request with method, path, status, and duration.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

// WriteJSON writes a JSON response with the given status code and data.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, statusCode int, message string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// ExtractPathParam extracts the last segment from a URL path.
// For example, "/api/v1/security-groups/abc123" returns "abc123".
func ExtractPathParam(path string, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")
	// Return everything up to the next "/" (or the whole string)
	if idx := strings.Index(trimmed, "/"); idx != -1 {
		return trimmed[:idx]
	}
	return trimmed
}

// ReadJSON reads and decodes JSON from request body.
func ReadJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// GetQueryParam extracts a query parameter from the request.
func GetQueryParam(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

// ExtractSubPathParam extracts a sub-path parameter after the resource ID.
// For example, "/api/v1/security-groups/abc123/rules/rule456" with prefix "/api/v1/security-groups/"
// and sub "rules/" returns "rule456".
func ExtractSubPathParam(path string, prefix string, sub string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	idx := strings.Index(trimmed, sub)
	if idx == -1 {
		return ""
	}
	remainder := trimmed[idx+len(sub):]
	remainder = strings.TrimPrefix(remainder, "/")
	if slashIdx := strings.Index(remainder, "/"); slashIdx != -1 {
		return remainder[:slashIdx]
	}
	return remainder
}
