package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                    string
	AppEnvironment          string
	DatabaseURL             string
	SessionSecret           string
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	GoogleOAuthRedirectURL  string
	AuthSuccessRedirectURL  string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("no .env file found")
	}

	port := os.Getenv("PORT")
	appEnvironment := os.Getenv("APP_ENVIRONMENT")
	databaseURL := os.Getenv("DATABASE_URL")
	sessionSecret := os.Getenv("SESSION_SECRET")
	googleOAuthClientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	googleOAuthClientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	googleOAuthRedirectURL := os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")
	authSuccessRedirectURL := os.Getenv("AUTH_SUCCESS_REDIRECT_URL")

	if port == "" {
		port = "8080"
	}

	if appEnvironment == "" {
		appEnvironment = "development"
	}

	if databaseURL == "" {
		if appEnvironment == "production" {
			log.Fatal("env var 'DATABASE_URL' is not set")
		}
		databaseURL = "file:./local.db"
	}

	if sessionSecret == "" {
		log.Fatal("env var 'SESSION_SECRET' is not set")
	}

	if googleOAuthClientID == "" {
		log.Fatal("env var 'GOOGLE_OAUTH_CLIENT_ID' is not set")
	}

	if googleOAuthClientSecret == "" {
		log.Fatal("env var 'GOOGLE_OAUTH_CLIENT_SECRET' is not set")
	}

	if googleOAuthRedirectURL == "" {
		log.Fatal("env var 'GOOGLE_OAUTH_REDIRECT_URL' is not set")
	}

	if authSuccessRedirectURL == "" {
		if appEnvironment == "production" {
			log.Fatal("env var 'AUTH_SUCCESS_REDIRECT_URL' is not set")
		}
		authSuccessRedirectURL = "/"
	}

	return &Config{
		Port:                    port,
		AppEnvironment:          appEnvironment,
		DatabaseURL:             databaseURL,
		SessionSecret:           sessionSecret,
		GoogleOAuthClientID:     googleOAuthClientID,
		GoogleOAuthClientSecret: googleOAuthClientSecret,
		GoogleOAuthRedirectURL:  googleOAuthRedirectURL,
		AuthSuccessRedirectURL:  authSuccessRedirectURL,
	}
}
