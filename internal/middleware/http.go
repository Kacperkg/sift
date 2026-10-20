// Package middleware contains HTTP concerns shared by every route.
package middleware

import "net/http"

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' http://127.0.0.1:8787; img-src 'self' data:; frame-ancestors 'none'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(writer, request)
	})
}

func CORS(appOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		localOrigin := "http://" + request.Host
		allowed := origin == "" || origin == appOrigin || origin == localOrigin
		if request.Method != http.MethodGet && request.Method != http.MethodHead && !allowed {
			http.Error(writer, "origin not allowed", http.StatusForbidden)
			return
		}
		if origin == appOrigin {
			writer.Header().Set("Access-Control-Allow-Origin", appOrigin)
		}
		if request.Method == http.MethodOptions {
			writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
