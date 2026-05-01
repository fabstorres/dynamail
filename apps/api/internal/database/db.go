package database

import (
	"database/sql"
	"time"

	"github.com/fabstorres/dynamail/apps/api/internal/config"
	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

type DatabaseClient struct {
	db *sql.DB
}

func NewClient(cfg *config.Config) (DatabaseService, error) {
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DatabaseClient{db: db}, nil
}

func (dc *DatabaseClient) Close() error {
	return dc.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id         TEXT PRIMARY KEY,
            email      TEXT UNIQUE NOT NULL,
            name       TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS sessions (
            id            TEXT PRIMARY KEY,
            user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            access_token  TEXT NOT NULL,
            refresh_token TEXT NOT NULL,
            expiry        DATETIME NOT NULL,
            created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
        );
    `)
	return err
}

func (dc *DatabaseClient) CreateUser(email, name string) (string, error) {
	id := generateID()

	_, err := dc.db.Exec(`
		INSERT INTO users (id, email, name)
		VALUES (?, ?, ?)
	`, id, email, name)

	if err != nil {
		return "", err
	}

	return id, nil
}
func (dc *DatabaseClient) GetUserByID(id string) (*User, error) {
	var user User
	err := dc.db.QueryRow(`
		SELECT id, email, name, created_at
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dc *DatabaseClient) GetUserByEmail(email string) (*User, error) {
	var user User
	err := dc.db.QueryRow(`
		SELECT id, email, name, created_at
		FROM users
		WHERE email = ?
	`, email).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dc *DatabaseClient) CreateSession(userID, accessToken, refreshToken string, expiry time.Time) (string, error) {
	// TODO: Sanitize inputs
	id := generateID()
	_, err := dc.db.Exec(`
		INSERT INTO sessions (id, user_id, access_token, refresh_token, expiry)
		VALUES (?, ?, ?, ?, ?)
	`, id, userID, accessToken, refreshToken, expiry.Format(time.RFC3339))
	return id, err
}

func (dc *DatabaseClient) GetSessionByID(id string) (*Session, error) {
	var session Session
	err := dc.db.QueryRow(`
		SELECT id, user_id, access_token, refresh_token, expiry, created_at
		FROM sessions
		WHERE id = ?
	`, id).Scan(&session.ID, &session.UserID, &session.AccessToken, &session.RefreshToken, &session.Expiry, &session.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (dc *DatabaseClient) GetSessionByUserID(userID string) ([]*Session, error) {
	rows, err := dc.db.Query(`
		SELECT id, user_id, access_token, refresh_token, expiry, created_at
		FROM sessions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.AccessToken, &s.RefreshToken, &s.Expiry, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (dc *DatabaseClient) DeleteSessionByID(id string) error {
	// TODO: Sanitize inputs
	_, err := dc.db.Exec(`
		DELETE FROM sessions
		WHERE id = ?
	`, id)
	return err
}

func (dc *DatabaseClient) DeleteUserByID(id string) error {
	_, err := dc.db.Exec(`
		DELETE FROM users
		WHERE id = ?
	`, id)
	return err
}

func (dc *DatabaseClient) UpsertUser(userID, email, name string) error {
	_, err := dc.db.Exec(`
		INSERT INTO users (id, email, name)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			email = excluded.email,
			name = excluded.name
	`, userID, email, name)
	return err
}

func (dc *DatabaseClient) UpsertSession(sessionID, userID, accessToken, refreshToken string, expiry time.Time) error {
	_, err := dc.db.Exec(`
		INSERT INTO sessions (id, user_id, access_token, refresh_token, expiry)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id = excluded.user_id,
			access_token = excluded.access_token,
			refresh_token = excluded.refresh_token,
			expiry = excluded.expiry
	`, sessionID, userID, accessToken, refreshToken, expiry.Format(time.RFC3339))
	return err
}

func generateID() string {
	return uuid.NewString()
}
