package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"text/template"

	"github.com/chai2010/webp"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nfnt/resize"
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
	verifyPage   string

	// Email templates for different languages
	verificationEmailTemplates VerificationEmailTemplates
)

// Email template structure for verification
type VerificationEmailTemplate struct {
	Subject      string `json:"subject"`
	Greeting     string `json:"greeting"`
	Message      string `json:"message"`
	CodeLabel    string `json:"code_label"`
	ExpiryNotice string `json:"expiry_notice"`
	Footer       string `json:"footer"`
}

// Email templates collection for verification
type VerificationEmailTemplates struct {
	Templates map[string]VerificationEmailTemplate `json:"templates"`
}

// Template data for verification email generation
type VerificationEmailTemplateData struct {
	UserName         string
	VerificationCode string
	Template         VerificationEmailTemplate
}

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
		BaseURL          string `json:"base_url"`
		VerificationPage string `json:"verification_page"`
	} `json:"app"`
}

type User struct {
	ID               int       `json:"id"`
	GoogleID         string    `json:"google_id"`
	Email            string    `json:"email"`
	Password         string    `json:"password,omitempty"` // Password is omitempty to not return it to client
	Name             string    `json:"name"`
	GivenName        string    `json:"given_name"`
	FamilyName       string    `json:"family_name"`
	Picture          string    `json:"picture"`                      // URL for Google users
	ProfileImageBlob string    `json:"profile_image_blob,omitempty"` // Base64 encoded WebP for manual signup
	Locale           string    `json:"locale"`
	VerifiedEmail    bool      `json:"verified_email"`
	VerificationCode string    `json:"verification_code,omitempty"` // Code for email verification
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SignupRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password,omitempty"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	PictureBase64 string `json:"picture_base64,omitempty"` // Base64 encoded image
	Locale        string `json:"locale"`
	VerifiedEmail bool   `json:"verified_email"`
}

type EmailCheckRequest struct {
	Email string `json:"email"`
}

type EmailCheckResponse struct {
	Exists bool `json:"exists"`
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
		verifyPage = "/verify-email"
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
		verifyPage = "/verify-email"
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
		verifyPage = "/verify-email"
		return
	}

	// Set configuration values
	smtpHost = config.SMTP.Host
	smtpPort = config.SMTP.Port
	smtpUsername = config.SMTP.Username
	smtpPassword = config.SMTP.Password
	fromEmail = config.SMTP.FromEmail
	appBaseURL = config.App.BaseURL
	verifyPage = config.App.VerificationPage

	log.Println("Configuration loaded successfully")
}

func loadVerificationEmailTemplates() {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct path to the verification email templates file
	templatesPath := filepath.Join(cwd, "verification_email_templates.json")

	// Check if templates file exists
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		log.Fatalf("Verification email templates file not found at: %s", templatesPath)
	}

	// Read and parse the templates file
	templatesFile, err := os.ReadFile(templatesPath)
	if err != nil {
		log.Fatalf("Error reading verification email templates file: %v", err)
	}

	if err := json.Unmarshal(templatesFile, &verificationEmailTemplates); err != nil {
		log.Fatalf("Error parsing verification email templates file: %v", err)
	}

	log.Printf("Verification email templates loaded for %d languages", len(verificationEmailTemplates.Templates))
}

// Get verification email template for language, fallback to English if not found
func getVerificationEmailTemplate(language string) VerificationEmailTemplate {
	// Normalize language code (e.g., "en-US" -> "en")
	lang := strings.Split(language, "-")[0]

	if template, exists := verificationEmailTemplates.Templates[lang]; exists {
		return template
	}

	// Fallback to English
	if template, exists := verificationEmailTemplates.Templates["en"]; exists {
		log.Printf("Language '%s' not found, using English fallback", language)
		return template
	}

	// If even English is not found, use hardcoded fallback
	log.Printf("No verification templates found, using hardcoded English fallback")
	return VerificationEmailTemplate{
		Subject:      "Hero Budget - Verify Your Email",
		Greeting:     "Hello {{.UserName}},",
		Message:      "Thank you for signing up with Hero Budget. To complete your registration, please enter the verification code below in the app:",
		CodeLabel:    "Your verification code:",
		ExpiryNotice: "This code will expire in 24 hours.",
		Footer:       "If you did not create an account with Hero Budget, please ignore this email.",
	}
}

func init() {
	// Load configuration
	loadConfig()

	// Load email templates
	loadVerificationEmailTemplates()

	var err error

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Make sure the table exists (should already be created by google_auth)
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
			verification_code TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Check if the password column exists, if not add it
	// A more reliable way to check for column existence
	rows, err := db.Query("PRAGMA table_info(users)")
	if err != nil {
		log.Fatalf("Failed to query table info: %v", err)
	}

	hasPasswordColumn := false
	hasProfileImageBlobColumn := false
	hasVerificationCodeColumn := false

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

		if name == "password" {
			hasPasswordColumn = true
		}
		if name == "profile_image_blob" {
			hasProfileImageBlobColumn = true
		}
		if name == "verification_code" {
			hasVerificationCodeColumn = true
		}
	}
	rows.Close()

	if !hasPasswordColumn {
		log.Println("Adding missing password column to users table")
		_, err = db.Exec("ALTER TABLE users ADD COLUMN password TEXT")
		if err != nil {
			log.Fatalf("Failed to add password column: %v", err)
		}
	}

	if !hasProfileImageBlobColumn {
		log.Println("Adding missing profile_image_blob column to users table")
		_, err = db.Exec("ALTER TABLE users ADD COLUMN profile_image_blob TEXT")
		if err != nil {
			log.Fatalf("Failed to add profile_image_blob column: %v", err)
		}
	}

	if !hasVerificationCodeColumn {
		log.Println("Adding missing verification_code column to users table")
		_, err = db.Exec("ALTER TABLE users ADD COLUMN verification_code TEXT")
		if err != nil {
			log.Fatalf("Failed to add verification_code column: %v", err)
		}
	}

	log.Println("Database connection established successfully")
}

func main() {
	// Setup HTTP handlers
	http.HandleFunc("/signup/register", corsMiddleware(handleSignup))
	http.HandleFunc("/signup/check-email", corsMiddleware(handleCheckEmail))
	http.HandleFunc("/signup/verify-email", corsMiddleware(handleVerifyEmail))
	http.HandleFunc("/signup/resend-verification", corsMiddleware(handleResendVerification))
	http.HandleFunc("/signup/check-verification", corsMiddleware(handleCheckVerification))
	http.HandleFunc("/ping", corsMiddleware(handlePing)) // Add ping endpoint for connectivity testing

	// Start the server
	port := 8082
	log.Printf("Signup service started on :%d", port)
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

	// Check if email exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", req.Email).Scan(&exists)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmailCheckResponse{Exists: exists})
}

// Helper function to generate a random verification code
func generateVerificationCode() string {
	// Generate a 6-digit numeric OTP code
	const digits = "0123456789"
	b := make([]byte, 6)
	for i := range b {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			// Fallback if there's an error with crypto/rand
			b[i] = digits[0]
			continue
		}
		b[i] = digits[randomIndex.Int64()]
	}
	return string(b)
}

// Process image: resize, compress, and convert to WebP
func processImage(base64Image string) (string, error) {
	// Extract the actual base64 content from the data URL
	base64Data := base64Image
	if idx := strings.Index(base64Image, ";base64,"); idx > 0 {
		base64Data = base64Image[idx+8:]
	}

	// Check if the base64 string is valid
	if len(base64Data) == 0 {
		return "", fmt.Errorf("empty base64 image data")
	}

	// Decode base64 image
	imgData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %v", err)
	}

	// Determine image format and decode
	imgReader := bytes.NewReader(imgData)
	img, format, err := image.Decode(imgReader)
	if err != nil {
		// Try to handle JPEG specifically if the generic decode fails
		imgReader.Seek(0, 0) // Reset reader
		img, err = jpeg.Decode(imgReader)
		if err != nil {
			// Try to handle PNG specifically if JPEG decode also fails
			imgReader.Seek(0, 0) // Reset reader
			img, err = png.Decode(imgReader)
			if err != nil {
				return "", fmt.Errorf("failed to decode image (tried generic, JPEG, and PNG formats): %v", err)
			}
			format = "png"
		} else {
			format = "jpeg"
		}
	}

	log.Printf("Image format: %s, size: %d KB", format, len(imgData)/1024)

	// Resize the image if it's too large
	// Calculate resize dimensions while maintaining aspect ratio
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var maxWidth uint = 800
	var maxHeight uint = 800

	if width > height && width > int(maxWidth) {
		img = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
	} else if height > int(maxHeight) {
		img = resize.Resize(0, maxHeight, img, resize.Lanczos3)
	}

	// Compress and convert to WebP
	var webpBuf bytes.Buffer
	err = webp.Encode(&webpBuf, img, &webp.Options{Quality: 80})
	if err != nil {
		return "", fmt.Errorf("failed to encode WebP: %v", err)
	}

	// Check if the compressed image is still too large (>100KB)
	compressedSize := webpBuf.Len()
	log.Printf("Compressed WebP size: %d KB", compressedSize/1024)

	// If still too large, compress more
	if compressedSize > 100*1024 {
		webpBuf.Reset()
		quality := 70
		for compressedSize > 100*1024 && quality > 10 {
			webpBuf.Reset()
			err = webp.Encode(&webpBuf, img, &webp.Options{Quality: float32(quality)})
			if err != nil {
				return "", fmt.Errorf("failed to encode WebP with quality %d: %v", quality, err)
			}
			compressedSize = webpBuf.Len()
			quality -= 10
			log.Printf("Recompressed WebP size: %d KB (quality: %d)", compressedSize/1024, quality)
		}
	}

	// Convert back to base64
	return base64.StdEncoding.EncodeToString(webpBuf.Bytes()), nil
}

// Send verification email with language support
func sendVerificationEmail(toEmail, verificationCode, userName, language string) error {
	// Validate email before attempting to send
	if toEmail == "" {
		return fmt.Errorf("cannot send verification email: email address is empty")
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
	emailTemplate := getVerificationEmailTemplate(language)

	// Log the values for debugging
	log.Printf("Sending verification email with OTP - Email: %s, Code: %s, Name: %s, Language: %s", toEmail, verificationCode, userName, language)

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
	templateData := VerificationEmailTemplateData{
		UserName:         userName,
		VerificationCode: verificationCode,
		Template:         emailTemplate,
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
        <p style="color: #4A154B; font-size: 16px; margin-bottom: 10px;">%s</p>
        <div style="background-color: #ffffff; padding: 20px; border-radius: 8px; font-size: 32px; letter-spacing: 5px; font-weight: bold; color: #6A1B9A; margin: 30px auto; max-width: 250px; box-shadow: 0 3px 5px rgba(106, 27, 154, 0.2);">
            %s
        </div>
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
		emailTemplate.CodeLabel,
		verificationCode,
		emailTemplate.ExpiryNotice,
		emailTemplate.Footer,
	)

	// Set the email body
	m.SetBody("text/html", emailBody)

	// Set up email sending
	d := gomail.NewDialer(smtpHost, smtpPort, smtpUsername, smtpPassword)

	// Send the email
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send verification email: %v", err)
	}

	log.Printf("Verification email with OTP sent successfully to %s in language: %s", toEmail, language)
	return nil
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Log request headers
	log.Println("Received signup request")

	// Read and log the raw request body for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Create a new reader from the bytes for JSON decoding
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Limit what we log to avoid huge outputs while still debugging the issue
	if len(bodyBytes) > 1000 {
		log.Printf("Request body (truncated): %s...", bodyBytes[:1000])
	} else {
		log.Printf("Request body: %s", bodyBytes)
	}

	var req SignupRequest
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

	// Log parsed request without sensitive data
	log.Printf("Parsed request: email=%s, name=%s, given_name=%s, family_name=%s, locale=%s, verified_email=%v, has_picture=%v",
		req.Email, req.Name, req.GivenName, req.FamilyName, req.Locale, req.VerifiedEmail, req.PictureBase64 != "")

	// Check if email already exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", req.Email).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking email: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if exists {
		log.Printf("User with email %s already exists", req.Email)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]string{"error": "User with this email already exists"})
		return
	}

	// Process the image if provided
	var processedImageBase64 string
	if req.PictureBase64 != "" {
		processedImage, err := processImage(req.PictureBase64)
		if err != nil {
			log.Printf("Failed to process image: %v", err)
			// Don't return error, just log and continue without the image
			if len(req.PictureBase64) > 100 {
				log.Printf("Image data preview (first 100 chars): %s...", req.PictureBase64[:100])
			} else {
				log.Printf("Image data (full): %s", req.PictureBase64)
			}
		} else {
			processedImageBase64 = processedImage
			log.Printf("Successfully processed and compressed image to WebP format")
		}
	}

	// Insert new user
	// In a real app, you would hash the password before storing
	log.Printf("Inserting new user: email=%s, name=%s, given_name=%s, family_name=%s",
		req.Email, req.Name, req.GivenName, req.FamilyName)

	// Set default values for empty fields
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s %s", req.GivenName, req.FamilyName)
	}

	givenName := req.GivenName
	if givenName == "" && req.Name != "" {
		// Try to extract first name from full name
		nameParts := strings.Split(req.Name, " ")
		if len(nameParts) > 0 {
			givenName = nameParts[0]
		}
	}

	familyName := req.FamilyName
	if familyName == "" && req.Name != "" {
		// Try to extract last name from full name
		nameParts := strings.Split(req.Name, " ")
		if len(nameParts) > 1 {
			familyName = strings.Join(nameParts[1:], " ")
		}
	}

	// Generate verification code
	verificationCode := generateVerificationCode()

	result, err := db.Exec(`
		INSERT INTO users (
			email, password, name, given_name, family_name, 
			picture, profile_image_blob, locale, verified_email,
			verification_code
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Email, req.Password, name, givenName,
		familyName, req.PictureBase64, processedImageBase64, req.Locale, false, // Set verified_email to false by default
		verificationCode,
	)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		log.Printf("Request data: email=%s, name=%s", req.Email, req.Name)
		http.Error(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusInternalServerError)
		return
	}

	userID, _ := result.LastInsertId()
	log.Printf("User created with ID: %d", userID)

	// Send verification email
	if smtpHost != "smtp.example.com" { // Only send if SMTP is configured
		// Log name for debugging
		log.Printf("User name for email: '%s'", name)

		// Make sure name is not empty to avoid formatting issues
		userNameForEmail := name
		if userNameForEmail == "" {
			userNameForEmail = "there" // Default fallback
		}

		err = sendVerificationEmail(req.Email, verificationCode, userNameForEmail, req.Locale)
		if err != nil {
			log.Printf("Warning: Failed to send verification email: %v", err)
			// Continue even if email sending fails
		} else {
			log.Printf("Verification email sent to %s", req.Email)
		}
	} else {
		log.Printf("SMTP not configured. Skipping verification email.")
	}

	// Fetch the inserted user to return
	var user User
	err = db.QueryRow(`
		SELECT id, email, name, given_name, family_name, picture, profile_image_blob, locale, verified_email, created_at, updated_at 
		FROM users WHERE id = ?`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.GivenName,
		&user.FamilyName,
		&user.Picture,
		&user.ProfileImageBlob,
		&user.Locale,
		&user.VerifiedEmail,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		log.Printf("Failed to fetch created user: %v", err)
		http.Error(w, "Failed to fetch created user", http.StatusInternalServerError)
		return
	}

	// Return the user object (without password and verification code)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
	log.Printf("User registration successful for ID: %d", user.ID)
}

// Add a new endpoint to handle email verification
func handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var code string
	var userID string
	var emailParam string

	if r.Method == "GET" {
		code = r.URL.Query().Get("code")
		userID = r.URL.Query().Get("user_id")
		emailParam = r.URL.Query().Get("email")
	} else {
		var req struct {
			Code   string `json:"code"`
			UserID string `json:"user_id"`
			Email  string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		code = req.Code
		userID = req.UserID
		emailParam = req.Email
	}

	if code == "" {
		http.Error(w, "Verification code is required", http.StatusBadRequest)
		return
	}

	// Log the verification attempt with additional parameters
	log.Printf("Attempting to verify email - Code: %s, UserID: %s, Email: %s",
		code, userID, emailParam)

	// First try to find the user with the verification code
	var dbUserID int
	var email string
	var verificationCode string
	var verified bool

	err := db.QueryRow(
		"SELECT id, email, verification_code, verified_email FROM users WHERE verification_code = ?",
		code,
	).Scan(&dbUserID, &email, &verificationCode, &verified)

	// If verification code not found but user_id or email is provided,
	// try to find the user with those parameters
	if err == sql.ErrNoRows && (userID != "" || emailParam != "") {
		log.Printf("Verification code not found. Trying to find user with userID or email")

		var query string
		var queryParams []interface{}

		if userID != "" {
			query = "SELECT id, email, verification_code, verified_email FROM users WHERE id = ?"
			queryParams = []interface{}{userID}
		} else {
			query = "SELECT id, email, verification_code, verified_email FROM users WHERE email = ?"
			queryParams = []interface{}{emailParam}
		}

		err = db.QueryRow(query, queryParams...).Scan(&dbUserID, &email, &verificationCode, &verified)

		if err == sql.ErrNoRows {
			log.Printf("User not found with userID=%s or email=%s", userID, emailParam)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		} else if err != nil {
			log.Printf("Database error looking up user: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// If we found the user but verification codes don't match
		if verificationCode != code {
			log.Printf("Found user but verification code doesn't match. Expected: %s, Got: %s",
				verificationCode, code)

			// Update verification code in database for next attempt
			_, err = db.Exec("UPDATE users SET verification_code = ? WHERE id = ?", code, dbUserID)
			if err != nil {
				log.Printf("Failed to update verification code: %v", err)
			} else {
				log.Printf("Updated verification code for user ID: %d", dbUserID)
			}

			// If user is already verified, we can still return success
			if verified {
				log.Printf("User ID: %d is already verified. Returning success.", dbUserID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": "Email already verified",
					"user_id": dbUserID,
					"email":   email,
				})
				return
			}

			// Otherwise, return error to trigger resend
			http.Error(w, "Invalid verification code", http.StatusNotFound)
			return
		}
	} else if err == sql.ErrNoRows {
		log.Printf("Invalid verification code: %s - User not found", code)
		http.Error(w, "Invalid verification code", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error looking up verification code: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if user is already verified
	if verified {
		log.Printf("User ID: %d is already verified. Returning success.", dbUserID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Email already verified",
			"user_id": dbUserID,
			"email":   email,
		})
		return
	}

	// Log that we found the user
	log.Printf("Found user ID: %d, email: %s for verification code: %s", dbUserID, email, code)

	// Update the user's verified_email status
	// Do NOT clear the verification code so the app can still verify it
	_, err = db.Exec(
		"UPDATE users SET verified_email = ? WHERE id = ?",
		true, dbUserID,
	)
	if err != nil {
		log.Printf("Failed to update user verification status: %v", err)
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	log.Printf("Email verified for user ID: %d, email: %s", dbUserID, email)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Email verification successful",
		"user_id": dbUserID,
		"email":   email,
	})
}

// Add a new endpoint to handle resending verification emails
func handleResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Locale string `json:"locale"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.UserID == "" && req.Email == "" {
		http.Error(w, "Either user_id or email is required", http.StatusBadRequest)
		return
	}

	log.Printf("Resend verification request for user_id=%s, email=%s", req.UserID, req.Email)

	// Look up the user
	var userID int
	var email string
	var name string
	var verificationCode string
	var userLocale string
	var query string
	var queryParams []interface{}

	if req.UserID != "" {
		// If we have a user ID, use that for lookup
		query = "SELECT id, email, name, verification_code, locale FROM users WHERE id = ?"
		queryParams = []interface{}{req.UserID}
	} else {
		// Otherwise use email
		query = "SELECT id, email, name, verification_code, locale FROM users WHERE email = ?"
		queryParams = []interface{}{req.Email}
	}

	err := db.QueryRow(query, queryParams...).Scan(&userID, &email, &name, &verificationCode, &userLocale)

	if err == sql.ErrNoRows {
		log.Printf("User not found for resend verification")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Use the locale from the request if provided, otherwise use the user's stored locale
	language := req.Locale
	if language == "" {
		language = userLocale
	}
	if language == "" {
		language = "en" // Default to English
	}

	// Check if verification_code is empty - user might already be verified
	if verificationCode == "" {
		// Generate a new verification code
		verificationCode = generateVerificationCode()

		// Update the user with the new verification code
		_, err = db.Exec(
			"UPDATE users SET verification_code = ? WHERE id = ?",
			verificationCode, userID,
		)

		if err != nil {
			log.Printf("Failed to update verification code: %v", err)
			http.Error(w, "Failed to update verification code", http.StatusInternalServerError)
			return
		}
	}

	// Send the verification email
	if smtpHost != "smtp.example.com" { // Only send if SMTP is configured
		err = sendVerificationEmail(email, verificationCode, name, language)
		if err != nil {
			log.Printf("Failed to send verification email: %v", err)
			http.Error(w, "Failed to send verification email", http.StatusInternalServerError)
			return
		}
		log.Printf("Verification email resent to %s", email)
	} else {
		log.Printf("SMTP not configured. Skipping verification email send.")
		http.Error(w, "SMTP not configured", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Verification email sent",
		"email":   email,
	})
}

// Add new endpoint to check verification status
func handleCheckVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.UserID == "" && req.Email == "" {
		http.Error(w, "Either user_id or email is required", http.StatusBadRequest)
		return
	}

	log.Printf("Check verification status for user_id=%s, email=%s", req.UserID, req.Email)

	// Look up the user
	var verified bool
	var query string
	var queryParams []interface{}

	if req.UserID != "" {
		// If we have a user ID, use that for lookup
		query = "SELECT verified_email FROM users WHERE id = ?"
		queryParams = []interface{}{req.UserID}
	} else {
		// Otherwise use email
		query = "SELECT verified_email FROM users WHERE email = ?"
		queryParams = []interface{}{req.Email}
	}

	err := db.QueryRow(query, queryParams...).Scan(&verified)

	if err == sql.ErrNoRows {
		log.Printf("User not found for verification check")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Return the verification status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"verified":       verified,
		"verified_email": verified,
	})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Signup service is running",
	})
}
