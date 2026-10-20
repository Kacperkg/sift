package handlers

import (
	"net/http"

	"github.com/Kacperkg/sift/internal/auth"
)

type AuthHandler struct {
	google    *auth.Google
	appOrigin string
}

func NewAuthHandler(google *auth.Google, appOrigin string) *AuthHandler {
	return &AuthHandler{google: google, appOrigin: appOrigin}
}

func (handler *AuthHandler) Status(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"connected": handler.google.Connected()})
}

func (handler *AuthHandler) Start(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet) {
		return
	}
	location, err := handler.google.AuthorizationURL()
	if err != nil {
		writeError(writer, err)
		return
	}
	http.Redirect(writer, request, location, http.StatusFound)
}

func (handler *AuthHandler) Callback(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet) {
		return
	}
	if err := handler.google.Complete(
		request.Context(), request.URL.Query().Get("state"), request.URL.Query().Get("code"), request.URL.Query().Get("error"),
	); err != nil {
		writeError(writer, err)
		return
	}
	http.Redirect(writer, request, handler.appOrigin+"/?connected=1", http.StatusFound)
}
