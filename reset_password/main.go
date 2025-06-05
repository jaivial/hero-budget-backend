package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/gomail.v2"
)

var (
	db *sql.DB
	// Email configuration - will be loaded from config.json
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	fromEmail    string
	appBaseURL   string
	resetPage    string

	// Email templates for different languages
	emailTemplates EmailTemplates
)

// Configuration structure
type Config struct {
	SMTP struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		FromEmail string `json:"from_email"`
	} `json:"smtp"`
	App struct {
		BaseURL   string `json:"base_url"`
		ResetPage string `json:"reset_page"`
	} `json:"app"`
}

// Email template structure
type EmailTemplate struct {
	Subject      string `json:"subject"`
	Greeting     string `json:"greeting"`
	Message      string `json:"message"`
	ButtonText   string `json:"button_text"`
	ExpiryNotice string `json:"expiry_notice"`
	Footer       string `json:"footer"`
}

// Email templates collection
type EmailTemplates struct {
	Templates map[string]EmailTemplate `json:"templates"`
}

// Template data for email generation
type EmailTemplateData struct {
	UserName  string
	ResetLink string
	Template  EmailTemplate
}

type User struct {
	ID               int       `json:"id"`
	GoogleID         string    `json:"google_id"`
	Email            string    `json:"email"`
	Password         string    `json:"password,omitempty"` // Password is omitempty to not return it to client
	Name             string    `json:"name"`
	GivenName        string    `json:"given_name"`
	FamilyName       string    `json:"family_name"`
	Picture          string    `json:"picture"`
	ProfileImageBlob string    `json:"profile_image_blob,omitempty"`
	Locale           string    `json:"locale"`
	VerifiedEmail    bool      `json:"verified_email"`
	ResetToken       string    `json:"reset_token,omitempty"`   // Token for password reset
	ResetExpires     time.Time `json:"reset_expires,omitempty"` // Expiration time for reset token
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ResetRequest struct {
	Email    string `json:"email"`
	Language string `json:"language"` // Added language field
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
	UserID      int    `json:"user_id"`
}

type EmailCheckRequest struct {
	Email string `json:"email"`
}

type EmailCheckResponse struct {
	Exists bool   `json:"exists"`
	UserID int    `json:"user_id"`
	Name   string `json:"name"`
}

func loadConfig() {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct path to the config file
	configPath := filepath.Join(cwd, "config.json")

	// Check if config file exists, if not use defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Println("Config file not found, using default values")
		smtpHost = "smtp.example.com"
		smtpPort = 587
		smtpUsername = "your-email@example.com"
		smtpPassword = "your-password"
		fromEmail = "your-email@example.com"
		appBaseURL = "http://localhost:3000"
		resetPage = "/reset-password"
		return
	}

	// Read and parse the config file
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config file: %v, using defaults", err)
		smtpHost = "smtp.example.com"
		smtpPort = 587
		smtpUsername = "your-email@example.com"
		smtpPassword = "your-password"
		fromEmail = "your-email@example.com"
		appBaseURL = "http://localhost:3000"
		resetPage = "/reset-password"
		return
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Printf("Error parsing config file: %v, using defaults", err)
		smtpHost = "smtp.example.com"
		smtpPort = 587
		smtpUsername = "your-email@example.com"
		smtpPassword = "your-password"
		fromEmail = "your-email@example.com"
		appBaseURL = "http://localhost:3000"
		resetPage = "/reset-password"
		return
	}

	// Set configuration values
	smtpHost = config.SMTP.Host
	smtpPort = config.SMTP.Port
	smtpUsername = config.SMTP.Username
	smtpPassword = config.SMTP.Password
	fromEmail = config.SMTP.FromEmail
	appBaseURL = config.App.BaseURL
	resetPage = config.App.ResetPage

	log.Println("Configuration loaded successfully")
}

func loadEmailTemplates() {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct path to the email templates file
	templatesPath := filepath.Join(cwd, "email_templates.json")

	// Check if templates file exists
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		log.Fatalf("Email templates file not found at: %s", templatesPath)
	}

	// Read and parse the templates file
	templatesFile, err := os.ReadFile(templatesPath)
	if err != nil {
		log.Fatalf("Error reading email templates file: %v", err)
	}

	if err := json.Unmarshal(templatesFile, &emailTemplates); err != nil {
		log.Fatalf("Error parsing email templates file: %v", err)
	}

	log.Printf("Email templates loaded for %d languages", len(emailTemplates.Templates))
}

// Get template for language, fallback to English if not found
func getEmailTemplate(language string) EmailTemplate {
	// Normalize language code (e.g., "en-US" -> "en")
	lang := strings.Split(language, "-")[0]

	if template, exists := emailTemplates.Templates[lang]; exists {
		return template
	}

	// Fallback to English
	if template, exists := emailTemplates.Templates["en"]; exists {
		log.Printf("Language '%s' not found, using English fallback", language)
		return template
	}

	// If even English is not found, use hardcoded fallback
	log.Printf("No templates found, using hardcoded English fallback")
	return EmailTemplate{
		Subject:      "Hero Budget - Reset Your Password",
		Greeting:     "Hello {{.UserName}},",
		Message:      "We received a request to reset your password for Hero Budget. Click the button below to create a new password:",
		ButtonText:   "Reset Password",
		ExpiryNotice: "This link will expire in 24 hours. If you did not request a password reset, please ignore this email.",
		Footer:       "If you did not request a password reset, please ignore this email or contact support if you have concerns.",
	}
}

func init() {
	// Load configuration
	loadConfig()

	// Load email templates
	loadEmailTemplates()

	var err error

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Check if database file exists, if not create it
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Printf("Database file doesn't exist, creating at: %s", dbPath)

		// Ensure the directory exists
		dbDir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}

		// Create database file
		file, err := os.Create(dbPath)
		if err != nil {
			log.Fatalf("Failed to create database file: %v", err)
		}
		file.Close()
	}

	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create users table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			google_id TEXT UNIQUE,
			email TEXT UNIQUE,
			password TEXT,
			name TEXT,
			given_name TEXT,
			family_name TEXT,
			picture TEXT,
			profile_image_blob TEXT,
			locale TEXT,
			verified_email BOOLEAN,
			reset_token TEXT,
			reset_expires DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Check if the reset_token and reset_expires columns exist, if not add them
	rows, err := db.Query("PRAGMA table_info(users)")
	if err != nil {
		log.Fatalf("Failed to query table info: %v", err)
	}

	hasResetTokenColumn := false
	hasResetExpiresColumn := false

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull bool
		var defaultValue interface{}
		var primaryKey bool

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			log.Fatalf("Failed to scan table info: %v", err)
		}

		if name == "reset_token" {
			hasResetTokenColumn = true
		}
		if name == "reset_expires" {
			hasResetExpiresColumn = true
		}
	}
	rows.Close()

	if !hasResetTokenColumn {
		log.Println("Adding missing reset_token column to users table")
		_, err = db.Exec("ALTER TABLE users ADD COLUMN reset_token TEXT")
		if err != nil {
			log.Fatalf("Failed to add reset_token column: %v", err)
		}
	}

	if !hasResetExpiresColumn {
		log.Println("Adding missing reset_expires column to users table")
		_, err = db.Exec("ALTER TABLE users ADD COLUMN reset_expires DATETIME")
		if err != nil {
			log.Fatalf("Failed to add reset_expires column: %v", err)
		}
	}

	log.Println("Database connection established successfully")
}

func main() {
	// Setup HTTP handlers
	http.HandleFunc("/reset-password/request", corsMiddleware(handleResetRequest))
	http.HandleFunc("/reset-password/validate-token", corsMiddleware(handleValidateToken))
	http.HandleFunc("/reset-password/check-email", corsMiddleware(handleCheckEmail))
	http.HandleFunc("/reset-password/update", corsMiddleware(handleUpdatePassword))
	http.HandleFunc("/ping", corsMiddleware(handlePing)) // Add ping endpoint for connectivity testing

	// Start the server
	port := 8086
	log.Printf("Reset Password service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the actual handler
		next(w, r)
	}
}

func handleCheckEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EmailCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if email exists and get user details
	var userID int
	var name string

	err := db.QueryRow("SELECT id, name FROM users WHERE email = ?", req.Email).Scan(&userID, &name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Email doesn't exist
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(EmailCheckResponse{Exists: false, UserID: 0, Name: ""})
			return
		}
		// Database error
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Email exists, return user details
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmailCheckResponse{Exists: true, UserID: userID, Name: name})
}

// Helper function to generate a random reset token
func generateResetToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Send reset password email with language support
func sendResetEmail(toEmail, resetToken, userName string, userID int, language string) error {
	// Validate email before attempting to send
	if toEmail == "" {
		return fmt.Errorf("cannot send reset email: email address is empty")
	}

	// Validate userName to prevent format errors
	if userName == "" {
		userName = "there" // Default fallback if name is empty
	}

	// Default to English if no language specified
	if language == "" {
		language = "en"
	}

	// Get the email template for the specified language
	emailTemplate := getEmailTemplate(language)

	// Log the values for debugging
	log.Printf("Sending reset email - Email: %s, Token: %s, Name: %s, UserID: %d, Language: %s", toEmail, resetToken, userName, userID, language)

	// Format a deep link URL that will be handled by the app
	// The format should be: herobudget://reset-password?token=RESET_TOKEN&user_id=USER_ID
	resetLink := fmt.Sprintf("herobudget://reset-password?token=%s&user_id=%d", resetToken, userID)
	log.Printf("Generated reset link: %s", resetLink)

	// Read the herobudgeticon.png image for embedding
	imgPath := filepath.Join("..", "..", "assets", "images", "herobudgeticon.png")
	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		log.Printf("Warning: Could not read icon file: %v", err)
		// Continue without the image if it can't be loaded
	}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", emailTemplate.Subject)

	// Create HTML with or without image
	var imageTag string
	if imgData != nil {
		// Embed the image and create HTML with the CID
		imgFilename := filepath.Base(imgPath)
		m.Embed(imgPath)
		imageTag = fmt.Sprintf(`<img src="cid:%s" alt="Hero Budget" style="max-width: 150px; margin: 20px 0;">`, imgFilename)
	} else {
		imageTag = ""
	}

	// Parse and execute the email template
	templateData := EmailTemplateData{
		UserName:  userName,
		ResetLink: resetLink,
		Template:  emailTemplate,
	}

	// Parse the greeting template
	greetingTmpl, err := template.New("greeting").Parse(emailTemplate.Greeting)
	if err != nil {
		log.Printf("Error parsing greeting template: %v", err)
		return fmt.Errorf("failed to parse greeting template: %v", err)
	}

	var greetingBuf bytes.Buffer
	if err := greetingTmpl.Execute(&greetingBuf, templateData); err != nil {
		log.Printf("Error executing greeting template: %v", err)
		return fmt.Errorf("failed to execute greeting template: %v", err)
	}

	// Build the email HTML body with the template data
	emailBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
</head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; color: #333333;">
    <div style="background-color: #F8E7FA; background: linear-gradient(135deg, #F8E7FA 0%%, #E6D0F0 100%%); border-radius: 12px; padding: 35px; text-align: center; box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);">
        %s
        <p style="margin-bottom: 20px; font-size: 18px; color: #4A154B; font-weight: 500;">%s</p>
        <p style="margin-bottom: 30px; color: #4A154B;">%s</p>
        <p style="text-align: center; margin: 30px 0;">
            <a href="%s" style="background-color: #6A1B9A; color: white; padding: 12px 30px; text-decoration: none; border-radius: 8px; font-weight: bold; display: inline-block; box-shadow: 0 3px 5px rgba(106, 27, 154, 0.3);">%s</a>
        </p>
        <p style="color: #4A154B; font-size: 14px;">%s</p>
    </div>
    <p style="color: #777777; font-size: 12px; text-align: center; margin-top: 20px;">
        %s
    </p>
</body>
</html>
`,
		emailTemplate.Subject,
		func() string {
			if imageTag != "" {
				return `<div style="filter: drop-shadow(0 4px 6px rgba(0, 0, 0, 0.1));">` + imageTag + `</div>`
			}
			return ""
		}(),
		greetingBuf.String(),
		emailTemplate.Message,
		resetLink,
		emailTemplate.ButtonText,
		emailTemplate.ExpiryNotice,
		emailTemplate.Footer,
	)

	// Set the email body
	m.SetBody("text/html", emailBody)

	// Set up email sending
	d := gomail.NewDialer(smtpHost, smtpPort, smtpUsername, smtpPassword)

	// Send the email
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send reset email: %v", err)
	}

	log.Printf("Reset email sent successfully to %s in language: %s", toEmail, language)
	return nil
}

func handleResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Log request headers
	log.Println("Received password reset request")

	// Read and log the raw request body for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Create a new reader from the bytes for JSON decoding
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var req ResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate email field
	if req.Email == "" {
		log.Printf("Email address is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email address is required"})
		return
	}

	log.Printf("Checking if email exists: %s", req.Email)

	// Check if email exists and get user details
	var userID int
	var name string

	err = db.QueryRow("SELECT id, name FROM users WHERE email = ?", req.Email).Scan(&userID, &name)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No user found with email: %s", req.Email)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "User with this email does not exist"})
			return
		}
		log.Printf("Database error checking email: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Generate reset token
	resetToken := generateResetToken()

	// Set expiration time (24 hours from now)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Update user with reset token
	_, err = db.Exec(
		"UPDATE users SET reset_token = ?, reset_expires = ? WHERE id = ?",
		resetToken, expiresAt, userID,
	)

	if err != nil {
		log.Printf("Failed to update user with reset token: %v", err)
		http.Error(w, "Failed to process reset request", http.StatusInternalServerError)
		return
	}

	// Send reset email
	if smtpHost != "smtp.example.com" { // Only send if SMTP is configured
		err = sendResetEmail(req.Email, resetToken, name, userID, req.Language)
		if err != nil {
			log.Printf("Warning: Failed to send reset email: %v", err)
			http.Error(w, "Failed to send reset email", http.StatusInternalServerError)
			return
		} else {
			log.Printf("Reset email sent to %s", req.Email)
		}
	} else {
		log.Printf("SMTP not configured. Skipping reset email.")
		http.Error(w, "SMTP not configured", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password reset email sent",
		"email":   req.Email,
		"user_id": userID,
	})
}

func handleValidateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	log.Printf("Validating reset token: %s", req.Token)

	// Check if token exists and is not expired
	var userID int
	var email string
	var expires time.Time

	err := db.QueryRow(
		"SELECT id, email, reset_expires FROM users WHERE reset_token = ?",
		req.Token,
	).Scan(&userID, &email, &expires)

	if err == sql.ErrNoRows {
		log.Printf("Invalid or expired token: %s", req.Token)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired token"})
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if token is expired
	if time.Now().After(expires) {
		log.Printf("Token expired: %s", req.Token)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Reset token has expired"})
		return
	}

	// Return success with user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"user_id": userID,
		"email":   email,
	})
}

func handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "Reset token is required", http.StatusBadRequest)
		return
	}

	if req.NewPassword == "" {
		http.Error(w, "New password is required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating password for user ID: %d with token: %s", req.UserID, req.Token)

	// Verify token is valid and get the user
	var userID int
	var currentPassword string
	var expires time.Time

	err := db.QueryRow(
		"SELECT id, password, reset_expires FROM users WHERE reset_token = ? AND id = ?",
		req.Token, req.UserID,
	).Scan(&userID, &currentPassword, &expires)

	if err == sql.ErrNoRows {
		log.Printf("Invalid token or user ID mismatch: %s, %d", req.Token, req.UserID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token or user ID"})
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if token is expired
	if time.Now().After(expires) {
		log.Printf("Token expired: %s", req.Token)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Reset token has expired"})
		return
	}

	// Check if new password is the same as current password
	if req.NewPassword == currentPassword {
		log.Printf("New password cannot be the same as current password")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "New password cannot be the same as current password"})
		return
	}

	// Update the password and clear reset token
	_, err = db.Exec(
		"UPDATE users SET password = ?, reset_token = NULL, reset_expires = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.NewPassword, userID,
	)

	if err != nil {
		log.Printf("Failed to update password: %v", err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	log.Printf("Password updated successfully for user ID: %d", userID)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password has been successfully updated",
	})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Reset Password service is running",
	})
}
