// Package handlers translates HTTP requests into service calls.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Kacperkg/sift/internal/apperrors"
)

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, apperrors.ErrUnauthorized):
		status = http.StatusUnauthorized
		message = err.Error()
	case errors.Is(err, apperrors.ErrNotFound):
		status = http.StatusNotFound
		message = err.Error()
	case errors.Is(err, apperrors.ErrConflict):
		status = http.StatusConflict
		message = err.Error()
	}
	http.Error(writer, message, status)
}

func methodAllowed(writer http.ResponseWriter, request *http.Request, methods ...string) bool {
	for _, method := range methods {
		if request.Method == method {
			return true
		}
	}
	http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	return false
}
