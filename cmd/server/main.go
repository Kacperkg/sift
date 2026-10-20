// Command server is Sift's local HTTP composition root.
package main

import (
	"log"
	"net/http"

	"github.com/Kacperkg/sift/internal/auth"
	"github.com/Kacperkg/sift/internal/config"
	"github.com/Kacperkg/sift/internal/handlers"
	"github.com/Kacperkg/sift/internal/middleware"
	"github.com/Kacperkg/sift/internal/repository"
	"github.com/Kacperkg/sift/internal/routes"
	"github.com/Kacperkg/sift/internal/services"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	applicationRepository, err := repository.NewApplicationRepository(settings.DataDir, settings.KeychainService)
	if err != nil {
		log.Fatal(err)
	}
	localAI, err := services.NewLocalAI(settings.OllamaURL, settings.OllamaModel, settings.LocalAIEnabled)
	if err != nil {
		log.Fatal(err)
	}
	google := auth.NewGoogle(
		settings.GoogleClientID, settings.GoogleClientSecret, settings.OAuthRedirectURL, settings.KeychainService,
	)
	applicationService := services.NewApplicationService(applicationRepository, google, localAI)
	mux := http.NewServeMux()
	routes.Register(mux, routes.Handlers{
		Applications: handlers.NewApplicationHandler(applicationService),
		Auth:         handlers.NewAuthHandler(google, settings.AppOrigin),
		AI:           handlers.NewAIHandler(localAI),
	}, settings.WebRoot)

	log.Printf("Sift is running at http://%s", settings.Address)
	log.Fatal(http.ListenAndServe(settings.Address, middleware.SecurityHeaders(middleware.CORS(settings.AppOrigin, mux))))
}
