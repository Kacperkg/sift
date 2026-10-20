// Package routes owns Sift's HTTP endpoint registration.
package routes

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/Kacperkg/sift/internal/handlers"
)

type Handlers struct {
	Applications *handlers.ApplicationHandler
	Auth         *handlers.AuthHandler
	AI           *handlers.AIHandler
}

func Register(mux *http.ServeMux, endpoints Handlers, webRoot string) {
	mux.HandleFunc("/api/health", handlers.Health)
	mux.HandleFunc("/api/auth/status", endpoints.Auth.Status)
	mux.HandleFunc("/api/applications", endpoints.Applications.Applications)
	mux.HandleFunc("/api/applications/status", endpoints.Applications.ApplicationStatus)
	mux.HandleFunc("/api/sync", endpoints.Applications.Sync)
	mux.HandleFunc("/api/sync/status", endpoints.Applications.SyncStatus)
	mux.HandleFunc("/api/blocked-roles", endpoints.Applications.BlockedRoles)
	mux.HandleFunc("/api/ai/status", endpoints.AI.Status)
	mux.HandleFunc("/auth/google", endpoints.Auth.Start)
	mux.HandleFunc("/auth/callback", endpoints.Auth.Callback)
	mux.Handle("/", spaHandler(webRoot))
}

func spaHandler(root string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := filepath.Join(root, filepath.Clean(request.URL.Path))
		if _, err := os.Stat(path); err != nil {
			request.URL.Path = "/"
		}
		files.ServeHTTP(writer, request)
	})
}
