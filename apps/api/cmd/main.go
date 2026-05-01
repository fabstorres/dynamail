package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/fabstorres/dynamail/apps/api/internal/auth"
	"github.com/fabstorres/dynamail/apps/api/internal/config"
)

func main() {
	cfg := config.Load() // This can panic if required env vars are not set
	r := chi.NewRouter()

	// Chi middlewares
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	authHandler := auth.NewHandler(cfg)

	// AUTH api routes
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authHandler.Login)
		r.Get("/callback", authHandler.Callback)
	})

	http.ListenAndServe(":3000", r)
}
