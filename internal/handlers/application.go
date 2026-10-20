package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Kacperkg/sift/internal/models"
	"github.com/Kacperkg/sift/internal/services"
)

type ApplicationHandler struct {
	service *services.ApplicationService
}

func NewApplicationHandler(service *services.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: service}
}

func (handler *ApplicationHandler) Applications(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet) {
		return
	}
	applications, err := handler.service.ListApplications()
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, applications)
}

func (handler *ApplicationHandler) ApplicationStatus(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodPost) {
		return
	}
	var update struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(request.Body, 2048)).Decode(&update); err != nil {
		http.Error(writer, "invalid outcome", http.StatusBadRequest)
		return
	}
	application, err := handler.service.CloseInterview(update.ID, update.Status)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, application)
}

func (handler *ApplicationHandler) BlockedRoles(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet, http.MethodPost) {
		return
	}
	if request.Method == http.MethodGet {
		blocks, err := handler.service.ListBlockedItems()
		if err != nil {
			writeError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, blocks)
		return
	}
	var update models.BlockUpdate
	if err := json.NewDecoder(io.LimitReader(request.Body, 2048)).Decode(&update); err != nil {
		http.Error(writer, "invalid block setting", http.StatusBadRequest)
		return
	}
	blocks, err := handler.service.UpdateBlockedItem(update)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, blocks)
}

func (handler *ApplicationHandler) Sync(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodPost) {
		return
	}
	applications, err := handler.service.Sync(request.Context(), strings.EqualFold(request.URL.Query().Get("mode"), "full"))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, applications)
}

func (handler *ApplicationHandler) SyncStatus(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet) {
		return
	}
	writeJSON(writer, http.StatusOK, handler.service.SyncStatus())
}
