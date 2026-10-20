package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Kacperkg/sift/internal/services"
)

type AIHandler struct {
	localAI *services.LocalAI
}

func NewAIHandler(localAI *services.LocalAI) *AIHandler {
	return &AIHandler{localAI: localAI}
}

func (handler *AIHandler) Status(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet, http.MethodPost) {
		return
	}
	if request.Method == http.MethodPost {
		var update struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(io.LimitReader(request.Body, 1024)).Decode(&update); err != nil {
			http.Error(writer, "invalid settings", http.StatusBadRequest)
			return
		}
		if update.Enabled {
			ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
			available, installed := handler.localAI.Availability(ctx)
			cancel()
			if !available {
				http.Error(writer, "Ollama is offline. Start Ollama first.", http.StatusServiceUnavailable)
				return
			}
			if !installed {
				http.Error(writer, "The configured Ollama model is not installed.", http.StatusServiceUnavailable)
				return
			}
		}
		handler.localAI.SetEnabled(update.Enabled)
	}
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	available, installed := handler.localAI.Availability(ctx)
	cancel()
	writeJSON(writer, http.StatusOK, map[string]any{
		"enabled": handler.localAI.IsEnabled(), "available": available,
		"modelInstalled": installed, "model": handler.localAI.Model(),
	})
}
