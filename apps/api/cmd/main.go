package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/fabstorres/dynamail/apps/api/internal/auth"
	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"github.com/fabstorres/dynamail/apps/api/internal/database"
	"github.com/fabstorres/dynamail/apps/api/internal/handler"
	dynamail "github.com/fabstorres/dynamail/apps/api/internal/middleware"
	"github.com/fabstorres/dynamail/apps/api/internal/session"
)

func main() {
	cfg := config.Load() // This can panic if required env vars are not set
	r := chi.NewRouter()

	// Chi middlewares
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Database
	db, err := database.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	sessionStore := session.NewStore(cfg)

	googleOAuthService := auth.NewGoogleOAuthService(cfg)
	authHandler := auth.NewHandler(cfg, sessionStore, db, googleOAuthService)

	// AUTH api routes
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", authHandler.Login)
		r.Get("/callback", authHandler.Callback)
		r.Post("/logout", authHandler.Logout)
	})

	sessionAuthMiddleware := dynamail.NewSessionAuthMiddleware(sessionStore, db, googleOAuthService)
	userHandler := handler.NewUserHandler(cfg, sessionStore, db)
	threadHandler := handler.NewThreadHandler()

	r.Route("/user", func(r chi.Router) {
		r.Use(sessionAuthMiddleware.Handle)
		r.Get("/me", userHandler.Me)
	})

	r.Route("/threads", func(r chi.Router) {
		r.Use(sessionAuthMiddleware.Handle)
		r.Get("/", threadHandler.List)
		r.Get("/{id}", threadHandler.Get)
		r.Patch("/{id}", threadHandler.Modify)
		r.Delete("/{id}/trash", threadHandler.Trash)
	})

	// TODO: move to listen and serve tls for production or handle x-forwarded-proto
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
