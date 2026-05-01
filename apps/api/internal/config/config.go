package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                    string
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	GoogleOAuthRedirectURL  string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("no .env file found")
	}

	port := os.Getenv("PORT")
	googleOAuthClientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	googleOAuthClientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	googleOAuthRedirectURL := os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")

	if port == "" {
		port = "8080"
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

	return &Config{
		Port:                    port,
		GoogleOAuthClientID:     googleOAuthClientID,
		GoogleOAuthClientSecret: googleOAuthClientSecret,
		GoogleOAuthRedirectURL:  googleOAuthRedirectURL,
	}
}
