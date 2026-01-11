package relay

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// MeResponse represents a /auth/me response
type MeResponse struct {
	Success bool   `json:"success"`
	User    *User  `json:"user,omitempty"`
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(AuthResponse{
		Success: false,
		Message: message,
	})
}

// handleRegister handles user registration requests
func handleRegister(w http.ResponseWriter, r *http.Request, authStore *AuthStore) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeJSONError(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := authStore.Register(req.Email, req.Password, req.Name)
	if err != nil {
		log.Printf("[Auth] Registration failed for %s: %v", req.Email, err)
		writeJSONError(w, "Registration failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[Auth] User registered: %s (ID: %d)", req.Email, user.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Registration successful",
		User:    user,
	})
}

// handleLogin handles user login requests and returns a JWT token
func handleLogin(w http.ResponseWriter, r *http.Request, authStore *AuthStore, cfg *Config) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		writeJSONError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Authenticate
	user, err := authStore.Login(req.Email, req.Password)
	if err != nil {
		log.Printf("[Auth] Login failed for %s: %v", req.Email, err)
		writeJSONError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := GenerateToken(user.ID, user.Email, cfg.JWTSecret)
	if err != nil {
		log.Printf("[Auth] Failed to generate token: %v", err)
		writeJSONError(w, "Login failed", http.StatusInternalServerError)
		return
	}

	log.Printf("[Auth] User logged in: %s (ID: %d)", req.Email, user.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Login successful",
		"user":    user,
		"token":   token,
	})
}

// handleLogout handles user logout requests
// With JWT access-only tokens, logout is handled client-side (delete token from localStorage)
func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// handleMe handles requests to get the current authenticated user using JWT
func handleMe(w http.ResponseWriter, r *http.Request, authStore *AuthStore, cfg *Config) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSONError(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	// Extract token from "Bearer <token>"
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		writeJSONError(w, "Invalid authorization header format", http.StatusUnauthorized)
		return
	}

	// Validate JWT token
	claims, err := ValidateToken(tokenString, cfg.JWTSecret)
	if err != nil {
		log.Printf("[Auth] Token validation failed: %v", err)
		writeJSONError(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Get user details
	user, err := authStore.GetUserByID(claims.UserID)
	if err != nil {
		writeJSONError(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MeResponse{
		Success: true,
		User:    user,
	})
}
