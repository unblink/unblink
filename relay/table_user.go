package relay

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
}

// table_user provides CRUD operations for users
type table_user struct {
	db *sql.DB
}

// NewUserTable creates a new user table accessor
func NewUserTable(db *sql.DB) *table_user {
	return &table_user{db: db}
}

// CreateUser creates a new user with email, password, and name
func (t *table_user) CreateUser(email, password, name string) (*User, error) {
	// Check if email already exists
	var existingID string
	err := t.db.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&existingID)
	if err == nil {
		return nil, errors.New("email already registered")
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check email existence: %w", err)
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Generate user ID
	userID := uuid.New().String()

	// Insert user
	_, err = t.db.Exec(
		"INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)",
		userID, email, string(hash), name,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return t.GetUserByID(userID)
}

// GetUserByEmail retrieves a user by email
func (t *table_user) GetUserByEmail(email string) (*User, error) {
	var user User
	err := t.db.QueryRow(`
		SELECT id, email, password_hash, name, created_at
		FROM users WHERE email = ?
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (t *table_user) GetUserByID(id string) (*User, error) {
	var user User
	err := t.db.QueryRow(`
		SELECT id, email, password_hash, name, created_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return &user, nil
}

// ValidatePassword validates a password against the user's hash
func (t *table_user) ValidatePassword(email, password string) (*User, error) {
	user, err := t.GetUserByEmail(email)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid password")
	}

	return user, nil
}

// UpdatePassword updates a user's password
func (t *table_user) UpdatePassword(userID string, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = t.db.Exec(
		"UPDATE users SET password_hash = ? WHERE id = ?",
		string(hash), userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}
