package handlers

import "net/http"

func Health(writer http.ResponseWriter, request *http.Request) {
	if !methodAllowed(writer, request, http.MethodGet) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"ok": true})
}
