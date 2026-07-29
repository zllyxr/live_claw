package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Envelope struct {
	RequestID string `json:"request_id"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
}

func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func OK(w http.ResponseWriter, requestID string, data any) {
	JSON(w, http.StatusOK, Envelope{RequestID: requestID, Code: 0, Message: "ok", Data: data})
}

func Error(w http.ResponseWriter, requestID string, status, code int, message string) {
	JSON(w, status, Envelope{RequestID: requestID, Code: code, Message: message})
}

func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("http panic", "request_id", RequestID(r.Context()), "panic", recovered)
				Error(w, RequestID(r.Context()), http.StatusInternalServerError, 500, "服务暂不可用")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
