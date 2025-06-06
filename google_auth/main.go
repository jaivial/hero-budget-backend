package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

var (
	googleOauthConfig *oauth2.Config
	db                *sql.DB
)

type User struct {
	ID            int       `json:"id"`
	GoogleID      string    `json:"google_id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	GivenName     string    `json:"given_name"`
	FamilyName    string    `json:"family_name"`
	Picture       string    `json:"picture"`
	Locale        string    `json:"locale"`
	VerifiedEmail bool      `json:"verified_email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func init() {
	// Load environment variables from .env file in parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Initialize OAuth config with environment variables
	googleOauthConfig = &oauth2.Config{
		ClientID:     getEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		ClientSecret: getEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),
		RedirectURL:  getEnvOrDefault("GOOGLE_REDIRECT_URL", "http://localhost:8081/auth/google/callback"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	// Validate required environment variables
	if googleOauthConfig.ClientID == "" {
		log.Fatal("GOOGLE_CLIENT_ID environment variable is required")
	}
	if googleOauthConfig.ClientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_SECRET environment variable is required")
	}

	var err error
	db, err = sql.Open("sqlite3", "./users.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create users table with expanded fields
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			google_id TEXT UNIQUE,
			email TEXT UNIQUE,
			name TEXT,
			given_name TEXT,
			family_name TEXT,
			picture TEXT,
			locale TEXT,
			verified_email BOOLEAN,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	http.HandleFunc("/auth/google", handleGoogleAuth)
	http.HandleFunc("/update/locale", handleUpdateLocale)
	http.HandleFunc("/health", handleHealth)

	// Registro de rutas y puertos
	log.Println("Registering routes:")
	log.Println("- POST /auth/google")
	log.Println("- POST /update/locale")
	log.Println("- GET /health")
	log.Println("Server started on :8081")

	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleGoogleAuth(w http.ResponseWriter, r *http.Request) {
	var data struct {
		IDToken      string `json:"idToken"`
		AccessToken  string `json:"accessToken"`
		DeviceLocale string `json:"deviceLocale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify the ID token
	payload, err := idtoken.Validate(r.Context(), data.IDToken, googleOauthConfig.ClientID)
	if err != nil {
		log.Printf("Failed to verify ID token: %v", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Extract user information from the verified payload
	user := User{
		GoogleID:      payload.Subject,
		Email:         payload.Claims["email"].(string),
		Name:          payload.Claims["name"].(string),
		GivenName:     payload.Claims["given_name"].(string),
		FamilyName:    payload.Claims["family_name"].(string),
		Picture:       payload.Claims["picture"].(string),
		VerifiedEmail: payload.Claims["email_verified"].(bool),
	}

	// Use device locale if provided, otherwise use Google's locale if available
	if data.DeviceLocale != "" {
		user.Locale = data.DeviceLocale
		log.Printf("Using device locale for user %s: %s", user.Email, user.Locale)
	} else if locale, ok := payload.Claims["locale"].(string); ok {
		user.Locale = locale
		log.Printf("Using Google-provided locale for user %s: %s", user.Email, user.Locale)
	} else {
		// Default locale if none is available
		user.Locale = "en-US"
		log.Printf("No locale available, defaulting to en-US for user %s", user.Email)
	}

	// Debug: Verify the locale is set correctly before database operations
	log.Printf("Final locale value before DB operations: '%s'", user.Locale)

	// Check if user exists in DB
	var existingUser User
	err = db.QueryRow(`
		SELECT id, email, name, given_name, family_name, picture, locale, verified_email, created_at, updated_at 
		FROM users WHERE google_id = ?`, user.GoogleID).Scan(
		&existingUser.ID,
		&existingUser.Email,
		&existingUser.Name,
		&existingUser.GivenName,
		&existingUser.FamilyName,
		&existingUser.Picture,
		&existingUser.Locale,
		&existingUser.VerifiedEmail,
		&existingUser.CreatedAt,
		&existingUser.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create new user
		log.Printf("Creating new user with locale: '%s'", user.Locale)
		result, err := db.Exec(`
			INSERT INTO users (
				google_id, email, name, given_name, family_name, 
				picture, locale, verified_email
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			user.GoogleID, user.Email, user.Name, user.GivenName,
			user.FamilyName, user.Picture, user.Locale, user.VerifiedEmail,
		)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		userID, _ := result.LastInsertId()
		user.ID = int(userID)
		log.Printf("Created new user with ID: %d, locale: '%s'", user.ID, user.Locale)
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	} else {
		// Update existing user
		log.Printf("Updating existing user with locale: '%s'", user.Locale)
		_, err = db.Exec(`
			UPDATE users SET 
				email = ?, name = ?, given_name = ?, family_name = ?,
				picture = ?, locale = ?, verified_email = ?, updated_at = CURRENT_TIMESTAMP
			WHERE google_id = ?`,
			user.Email, user.Name, user.GivenName, user.FamilyName,
			user.Picture, user.Locale, user.VerifiedEmail, user.GoogleID,
		)
		if err != nil {
			log.Printf("Failed to update user: %v", err)
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}
		user.ID = existingUser.ID
		user.CreatedAt = existingUser.CreatedAt
		log.Printf("Updated user ID: %d, changed locale from '%s' to '%s'", user.ID, existingUser.Locale, user.Locale)
	}

	// Verify the user's locale one final time before sending response
	log.Printf("User locale in final response: '%s'", user.Locale)

	// Return user information
	json.NewEncoder(w).Encode(user)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		sendErrorResponse(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Google Auth service is healthy", map[string]string{
		"status":    "healthy",
		"service":   "google_auth",
		"port":      "8081",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: false,
		Message: message,
	})
}

// UpdateLocaleRequest representa la solicitud para actualizar el idioma del usuario
type UpdateLocaleRequest struct {
	UserID int    `json:"user_id"`
	Locale string `json:"locale"`
}

// handleUpdateLocale maneja las solicitudes para actualizar el idioma de un usuario
func handleUpdateLocale(w http.ResponseWriter, r *http.Request) {
	// Solo permitir método POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decodificar la solicitud JSON
	var req UpdateLocaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decodificando la solicitud: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validar los datos de la solicitud
	if req.UserID <= 0 {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if req.Locale == "" {
		http.Error(w, "Locale cannot be empty", http.StatusBadRequest)
		return
	}

	// Actualizar el idioma en la base de datos
	err := updateUserLocale(req.UserID, req.Locale)
	if err != nil {
		log.Printf("Error actualizando el idioma: %v", err)
		http.Error(w, "Failed to update locale", http.StatusInternalServerError)
		return
	}

	// Enviar respuesta de éxito
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Locale updated successfully",
		"locale":  req.Locale,
	})
}

// updateUserLocale actualiza el campo 'locale' en la tabla 'users' para el usuario especificado
func updateUserLocale(userID int, locale string) error {
	// Actualizar el registro en la base de datos
	_, err := db.Exec(`
		UPDATE users 
		SET locale = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, locale, userID)

	if err != nil {
		log.Printf("Error SQL al actualizar el idioma para el usuario %d: %v", userID, err)
		return err
	}

	log.Printf("Idioma actualizado para el usuario %d: %s", userID, locale)
	return nil
}
