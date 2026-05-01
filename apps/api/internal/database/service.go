package database

import "time"

type User struct {
	ID        string
	Email     string
	Name      string
	CreatedAt string
}

type Session struct {
	ID           string
	UserID       string
	AccessToken  string
	RefreshToken string
	Expiry       string
	CreatedAt    string
}

type DatabaseService interface {
	Close() error

	CreateUser(email, name string) (string, error)
	CreateSession(userID, accessToken, refreshToken string, expiry time.Time) (string, error)
	GetUserByID(userID string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	GetSessionByID(sessionID string) (*Session, error)
	GetSessionByUserID(userID string) ([]*Session, error)
	DeleteSessionByID(sessionID string) error
	DeleteUserByID(userID string) error
	UpsertUser(userID, email, name string) error
	UpsertSession(sessionID, userID, accessToken, refreshToken string, expiry time.Time) error
}
