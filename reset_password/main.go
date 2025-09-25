package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/gomail.v2"
)

var (
	// Database connection for user data and reset tokens
	db *sql.DB
	// Context for database operations
	ctx = context.Background()
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
	Subtitle     string `json:"subtitle"`
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
	templatesPath := filepath.Join(cwd, "password_reset_email_templates.json")

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
	log.Printf("🌍 Starting language resolution for: '%s'", language)

	// Step 1: Try exact match (e.g., "ca_ES")
	if template, exists := emailTemplates.Templates[language]; exists {
		log.Printf("✅ Step 1 - Exact match found: %s", language)
		return template
	}
	log.Printf("❌ Step 1 - No exact match for: %s", language)

	// Step 2: Try with underscore replacement (e.g., "ca-ES" -> "ca_ES")
	normalizedLang := strings.ReplaceAll(language, "-", "_")
	if normalizedLang != language {
		if template, exists := emailTemplates.Templates[normalizedLang]; exists {
			log.Printf("✅ Step 2 - Found with underscore: %s", normalizedLang)
			return template
		}
		log.Printf("❌ Step 2 - No match with underscore: %s", normalizedLang)
	}

	// Step 3: Try with dash replacement (e.g., "ca_ES" -> "ca-ES")
	dashLang := strings.ReplaceAll(language, "_", "-")
	if dashLang != language {
		if template, exists := emailTemplates.Templates[dashLang]; exists {
			log.Printf("✅ Step 3 - Found with dash: %s", dashLang)
			return template
		}
		log.Printf("❌ Step 3 - No match with dash: %s", dashLang)
	}

	// Step 4: Try language code only (e.g., "ca_ES" -> "ca")
	baseLang := strings.Split(strings.Split(language, "_")[0], "-")[0]
	if baseLang != language && baseLang != normalizedLang {
		if template, exists := emailTemplates.Templates[baseLang]; exists {
			log.Printf("✅ Step 4 - Found base language: %s", baseLang)
			return template
		}
		log.Printf("❌ Step 4 - No match for base language: %s", baseLang)
	}

	// Step 5: Try lowercase version
	lowerLang := strings.ToLower(language)
	if lowerLang != language {
		if template, exists := emailTemplates.Templates[lowerLang]; exists {
			log.Printf("✅ Step 5 - Found lowercase: %s", lowerLang)
			return template
		}
		log.Printf("❌ Step 5 - No match for lowercase: %s", lowerLang)
	}

	// Step 6: Try common language mappings
	commonMappings := map[string]string{
		"en": "en_US", "english": "en_US",
		"es": "es_ES", "spanish": "es_ES", "esp": "es_ES",
		"fr": "fr_FR", "french": "fr_FR", "fra": "fr_FR",
		"de": "de_DE", "german": "de_DE", "deu": "de_DE",
		"it": "it_IT", "italian": "it_IT", "ita": "it_IT",
		"pt": "pt_PT", "portuguese": "pt_PT", "por": "pt_PT",
		"ca": "ca_ES", "catalan": "ca_ES", "cat": "ca_ES",
		"zh": "zh_CN", "chinese": "zh_CN",
		"ja": "ja_JP", "japanese": "ja_JP",
		"ko": "ko_KR", "korean": "ko_KR",
		"ar": "ar_SA", "arabic": "ar_SA",
		"ru": "ru_RU", "russian": "ru_RU",
		"hi": "hi_IN", "hindi": "hi_IN",
	}

	if mappedLang, exists := commonMappings[strings.ToLower(baseLang)]; exists {
		if template, exists := emailTemplates.Templates[mappedLang]; exists {
			log.Printf("✅ Step 6 - Found via mapping %s -> %s", baseLang, mappedLang)
			return template
		}
		log.Printf("❌ Step 6 - Mapped language not available: %s", mappedLang)
	}

	// Step 7: Try English variants as fallback
	englishVariants := []string{"en_US", "en_GB", "en_CA"}
	for _, variant := range englishVariants {
		if template, exists := emailTemplates.Templates[variant]; exists {
			log.Printf("✅ Step 7 - Found English variant: %s", variant)
			return template
		}
	}
	log.Printf("❌ Step 7 - No English variants found")

	// Step 8: Hardcoded fallback with new subtitle field
	log.Printf("⚠️ Step 8 - Using hardcoded English fallback")
	return EmailTemplate{
		Subject:      "Hero Budget - Reset Your Password",
		Subtitle:     "Password Reset",
		Greeting:     "Hello {{.UserName}},",
		Message:      "We received a request to reset your password for Hero Budget. Click the button below to create a new password:",
		ButtonText:   "Reset Password",
		ExpiryNotice: "This link will expire in 24 hours. If you did not request a password reset, please ignore this email.",
		Footer:       "If you did not request a password reset, please ignore this email or contact support if you have concerns.",
	}
}

func init() {
	// Parse command line flags
	devMode := flag.Bool("dev", false, "Run in development mode")
	prodMode := flag.Bool("produccion", false, "Run in production mode")
	flag.Parse()

	// Load environment variables from .env file in parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Load configuration
	loadConfig()

	// Load email templates
	loadEmailTemplates()

	// Determine database path based on environment flag
	var dbPath string
	if *prodMode {
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)
	} else if *devMode {
		dbPath = getEnvOrDefault("DB_DEV_PATH", "../google_auth/users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)
	} else {
		// Default to development mode if no flag specified
		dbPath = getEnvOrDefault("DB_DEV_PATH", "../google_auth/users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)
	}

	var err error
	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	// CENTRALIZED SCHEMA: DDL operations moved to database_schema.sql
	// Tables are now created by centralized database initialization
	log.Println("✅ Using centralized database schema - no local DDL operations")

	log.Println("Database connection established successfully")

	log.Println("Reset Password service initialized successfully")
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

	err := db.QueryRow("SELECT id, name FROM users WHERE email = ? AND type = 'email'", req.Email).Scan(&userID, &name)
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

	// Also create URL encoded version for better email client compatibility
	encodedResetLink := fmt.Sprintf("herobudget%%3A//reset-password%%3Ftoken%%3D%s%%26user_id%%3D%d", resetToken, userID)
	log.Printf("URL encoded reset link: %s", encodedResetLink)

	// Image hardcoded in template - updated version

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", emailTemplate.Subject)

	// Parse and execute the greeting template
	greetingTmpl, err := template.New("greeting").Parse(emailTemplate.Greeting)
	if err != nil {
		log.Printf("Error parsing greeting template: %v", err)
		return fmt.Errorf("failed to parse greeting template: %v", err)
	}

	var greetingBuf bytes.Buffer
	if err := greetingTmpl.Execute(&greetingBuf, EmailTemplateData{
		UserName:  userName,
		ResetLink: resetLink,
		Template:  emailTemplate,
	}); err != nil {
		log.Printf("Error executing greeting template: %v", err)
		return fmt.Errorf("failed to execute greeting template: %v", err)
	}

	// Create beautiful email template with signup service styling
	emailBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light only">
    <meta name="supported-color-schemes" content="light only">
    <title>%s</title>
    <style>
        /* Force light theme for all email clients */
        :root { color-scheme: light only !important; }

        /* Reset and base styles */
        * {
            box-sizing: border-box !important;
            -webkit-text-size-adjust: 100%% !important;
            -ms-text-size-adjust: 100%% !important;
            -webkit-font-smoothing: antialiased !important;
            -moz-osx-font-smoothing: grayscale !important;
        }

        /* Mobile responsiveness */
        @media only screen and (max-width: 480px) {
            .main-container { margin: 10px !important; padding: 20px !important; }
            .hero-title { font-size: 28px !important; }
            .reset-button { font-size: 14px !important; padding: 14px 24px !important; }
        }

        /* Button styling that works in all email clients */
        .reset-button {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%) !important;
            color: #ffffff !important;
            text-decoration: none !important;
            font-weight: bold !important;
            font-size: 16px !important;
            text-transform: uppercase !important;
            letter-spacing: 1px !important;
            font-family: Arial, sans-serif !important;
            display: inline-block !important;
            padding: 16px 32px !important;
            line-height: 1.2 !important;
            border-radius: 12px !important;
            border: 2px solid #ffffff !important;
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3) !important;
            text-align: center !important;
        }
    </style>
</head>
<body style="margin: 0 !important; padding: 0 !important; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif !important; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%) !important; line-height: 1.6 !important; min-height: 100vh !important;">
    <table role="presentation" style="width: 100%%; margin: 0; padding: 0; background: transparent;">
        <tr>
            <td style="padding: 20px 15px;">
                <div class="main-container" style="max-width: 600px; margin: 0 auto; background: transparent; border-radius: 16px; overflow: hidden; padding: 30px 20px;">

                    <!-- Header with logo -->
                    <div style="text-align: center; margin-bottom: 30px;">
                        <img src="https://herobudgetapp.jaimedigitalstudio.com/herobudgeticon.png" alt="Hero Budget" style="display: block; margin: 0 auto 20px auto; width: 90px; height: auto; max-width: 100%%;" />
                        <h1 class="hero-title" style="margin: 15px 0 5px 0; font-size: 36px; font-weight: 900; text-transform: uppercase; letter-spacing: 2px; background: linear-gradient(135deg, #fff 0%%, #e0e7ff 100%%); -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent; text-shadow: 0 4px 8px rgba(0,0,0,0.1); color: #ffffff;">HERO BUDGET</h1>
                        <p style="margin: 0; font-size: 18px; color: rgba(255,255,255,0.9); font-weight: 600;">%s</p>
                    </div>

                    <!-- Main content -->
                    <div style="text-align: center; margin-bottom: 30px;">
                        <div style="margin-bottom: 25px;">
                            <h2 style="margin: 0 0 12px 0; font-size: 24px; font-weight: 700; color: #ffffff; line-height: 1.3; text-shadow: 0 2px 4px rgba(0,0,0,0.1);">%s</h2>
                            <p style="margin: 0 0 20px 0; font-size: 16px; color: rgba(255,255,255,0.9); line-height: 1.5;">%s</p>
                        </div>

                        <!-- Reset button container -->
                        <div style="margin: 25px auto; text-align: center;">
                            <a href="%s" class="reset-button" style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%) !important; color: #ffffff !important; text-decoration: none !important; font-weight: bold !important; font-size: 16px !important; text-transform: uppercase !important; letter-spacing: 1px !important; font-family: Arial, sans-serif !important; display: inline-block !important; padding: 16px 32px !important; line-height: 1.2 !important; border-radius: 12px !important; border: 2px solid #ffffff !important; box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3) !important; text-align: center !important;">%s</a>
                        </div>
                    </div>

                    <!-- Footer -->
                    <div style="text-align: center; padding-top: 20px; border-top: 1px solid rgba(255,255,255,0.2);">
                        <p style="margin: 0 0 15px 0; font-size: 14px; color: rgba(255,255,255,0.9); line-height: 1.4;">
                            %s
                        </p>
                        <p style="margin: 0; font-size: 12px; color: rgba(255,255,255,0.7);">
                            © 2025 Hero Budget. All rights reserved.
                        </p>
                    </div>
                </div>
            </td>
        </tr>
    </table>
</body>
</html>
`,
		emailTemplate.Subject,      // %s in <title>
		emailTemplate.Subtitle,     // %s for subtitle below logo
		greetingBuf.String(),       // %s for greeting
		emailTemplate.Message,      // %s for main message
		resetLink,                  // %s for button href
		emailTemplate.ButtonText,   // %s for button text
		emailTemplate.ExpiryNotice, // %s for expiry notice
	)

	// Set HTML as primary content with proper headers for better client compatibility
	m.SetHeader("MIME-Version", "1.0")
	m.SetHeader("Content-Type", "text/html; charset=UTF-8")
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

	err = db.QueryRow("SELECT id, name FROM users WHERE email = ? AND type = 'email'", req.Email).Scan(&userID, &name)
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
		"UPDATE users SET reset_token = ?, reset_expires = ? WHERE id = ? AND type = 'email'",
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
		"SELECT id, email, reset_expires FROM users WHERE reset_token = ? AND type = 'email'",
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
	var expiresStr sql.NullString // Changed to handle NULL values and string scanning

	err := db.QueryRow(
		"SELECT id, password, reset_expires FROM users WHERE reset_token = ? AND id = ? AND type = 'email'",
		req.Token, req.UserID,
	).Scan(&userID, &currentPassword, &expiresStr)

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
	if expiresStr.Valid && expiresStr.String != "" {
		// Parse the datetime string from database
		expires, err := time.Parse("2006-01-02 15:04:05", expiresStr.String)
		if err != nil {
			// Try alternative format if the first one fails
			expires, err = time.Parse(time.RFC3339, expiresStr.String)
			if err != nil {
				log.Printf("Failed to parse expiration time: %v", err)
				http.Error(w, "Invalid token expiration format", http.StatusInternalServerError)
				return
			}
		}

		if time.Now().After(expires) {
			log.Printf("Token expired: %s", req.Token)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Reset token has expired"})
			return
		}
	}

	// Check if new password is the same as current password
	// Skip comparison if current password is empty/null (e.g., for OAuth users)
	if currentPassword != "" && req.NewPassword == currentPassword {
		log.Printf("New password cannot be the same as current password")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "New password cannot be the same as current password"})
		return
	}

	// Update the password and clear reset token
	_, err = db.Exec(
		"UPDATE users SET password = ?, reset_token = NULL, reset_expires = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND type = 'email'",
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

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
