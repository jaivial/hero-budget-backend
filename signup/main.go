package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
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
	"github.com/joho/godotenv"
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
	AppleID          string    `json:"apple_id"`
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
	Type             string    `json:"type"`                        // Type of authentication: 'email', 'google', 'apple'
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
	// Language code mapping and fallback system for comprehensive locale support
	log.Printf("Resolving email template for language: '%s'", language)

	// Step 1: Try exact match first (e.g., "en_US", "es_ES")
	if template, exists := verificationEmailTemplates.Templates[language]; exists {
		log.Printf("Found exact language match for: '%s'", language)
		return template
	}

	// Step 2: Normalize language codes - handle both dash and underscore formats
	normalizedLang := language
	if strings.Contains(language, "-") {
		// Convert "en-US" to "en_US" format
		normalizedLang = strings.Replace(language, "-", "_", -1)
		if template, exists := verificationEmailTemplates.Templates[normalizedLang]; exists {
			log.Printf("Found normalized language match for: '%s' -> '%s'", language, normalizedLang)
			return template
		}
	}

	// Step 3: Language-only fallbacks - extract base language code
	var baseLang string
	if strings.Contains(normalizedLang, "_") {
		baseLang = strings.Split(normalizedLang, "_")[0]
	} else if strings.Contains(normalizedLang, "-") {
		baseLang = strings.Split(normalizedLang, "-")[0]
	} else {
		baseLang = normalizedLang
	}

	// Step 4: Regional fallback mapping for common language variants
	languageFallbacks := map[string][]string{
		"en": {"en_US", "en_GB", "en_CA"},
		"es": {"es_ES", "es_MX"},
		"de": {"de_DE", "de_CH"},
		"fr": {"fr_FR", "fr_CA"},
		"pt": {"pt_PT", "pt_BR"},
		"zh": {"zh_CN"},
		"ar": {"ar_SA"},
		"hi": {"hi_IN"},
		"he": {"he_IL"},
		"ko": {"ko_KR"},
		"ja": {"ja_JP"},
		"it": {"it_IT"},
		"ru": {"ru_RU"},
		"nl": {"nl_NL"},
		"sv": {"sv_SE"},
		"no": {"no_NO"},
		"da": {"da_DK"},
		"fi": {"fi_FI"},
		"pl": {"pl_PL"},
		"cs": {"cs_CZ"},
		"tr": {"tr_TR"},
		"uk": {"uk_UA"},
		"vi": {"vi_VN"},
		"th": {"th_TH"},
		"id": {"id_ID"},
		"ca": {"ca_ES"},
	}

	// Try language fallbacks
	if fallbacks, exists := languageFallbacks[baseLang]; exists {
		for _, fallback := range fallbacks {
			if template, exists := verificationEmailTemplates.Templates[fallback]; exists {
				log.Printf("Found language fallback for: '%s' -> '%s'", language, fallback)
				return template
			}
		}
	}

	// Step 5: Try legacy format fallbacks (for backward compatibility)
	legacyMappings := map[string]string{
		"en": "en_US",
		"es": "es_ES",
		"de": "de_DE",
		"fr": "fr_FR",
		"pt": "pt_PT",
		"it": "it_IT",
		"ru": "ru_RU",
		"ja": "ja_JP",
		"zh": "zh_CN",
		"nl": "nl_NL",
		"da": "da_DK",
		"hi": "hi_IN",
	}

	if mappedLang, exists := legacyMappings[baseLang]; exists {
		if template, exists := verificationEmailTemplates.Templates[mappedLang]; exists {
			log.Printf("Found legacy mapping for: '%s' -> '%s'", language, mappedLang)
			return template
		}
	}

	// Step 6: Final fallback to English (en_US)
	if template, exists := verificationEmailTemplates.Templates["en_US"]; exists {
		log.Printf("Language '%s' not found, using English (en_US) fallback", language)
		return template
	}

	// Step 7: If even en_US is not found, try en_GB or en_CA
	englishFallbacks := []string{"en_GB", "en_CA"}
	for _, englishLang := range englishFallbacks {
		if template, exists := verificationEmailTemplates.Templates[englishLang]; exists {
			log.Printf("Using English fallback: %s for language '%s'", englishLang, language)
			return template
		}
	}

	// Step 8: Absolute fallback - hardcoded English template
	log.Printf("No verification templates found at all, using hardcoded English fallback for: '%s'", language)
	return VerificationEmailTemplate{
		Subject:      "Hero Budget - Verify Your Email",
		Greeting:     "Hello {{.UserName}},",
		Message:      "Thank you for signing up with Hero Budget. To complete your registration, please enter the verification code below in the app:",
		CodeLabel:    "Your verification code:",
		ExpiryNotice: "This code will expire in 24 hours.",
		Footer:       "If you did not create an account with Hero Budget, please ignore this email.",
	}
}

func migrateTableStructure(db *sql.DB) error {
	log.Println("🔍 Checking if table structure migration is needed...")
	
	// PROTECCIÓN CRÍTICA: Verificar usuarios existentes antes de cualquier migración
	userCount, err := getUserCount(db)
	if err != nil {
		log.Printf("⚠️ Warning: Could not get user count before migration: %v", err)
		userCount = 0
	}
	log.Printf("📊 Current users in database: %d", userCount)
	
	// Backup específico de usuarios tipo email antes de migración
	emailUsers, err := getEmailTypeUsers(db)
	if err != nil {
		log.Printf("⚠️ Warning: Could not backup email users: %v", err)
	} else {
		log.Printf("🔐 Found %d users with type='email' to protect", len(emailUsers))
	}

	// Check if the current table has the old structure (email UNIQUE)
	// We'll check the table schema to see if it has the problematic constraint
	rows, err := db.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name='users'")
	if err != nil {
		return fmt.Errorf("failed to query table schema: %v", err)
	}
	defer rows.Close()

	var tableSQL string
	if rows.Next() {
		if err := rows.Scan(&tableSQL); err != nil {
			return fmt.Errorf("failed to scan table schema: %v", err)
		}
	}

	// Check if the table has the old structure (email TEXT UNIQUE without compound constraint)
	hasOldStructure := strings.Contains(tableSQL, "email TEXT UNIQUE") && !strings.Contains(tableSQL, "UNIQUE(email, type)")

	if !hasOldStructure {
		log.Println("✅ Table structure is already up to date - no migration needed")
		log.Printf("🔐 Users remain safe: %d total users preserved", userCount)
		return nil
	}

	log.Println("🚨 CRITICAL: Migrating table structure from old (email UNIQUE) to new (email, type UNIQUE)...")
	log.Printf("🔐 Protecting %d users during migration, including %d email-type users", userCount, len(emailUsers))

	// Begin transaction for safe migration
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create new table with correct structure
	_, err = tx.Exec(`
		CREATE TABLE users_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			google_id TEXT UNIQUE,
			apple_id TEXT,
			email TEXT,
			password TEXT,
			name TEXT,
			given_name TEXT,
			family_name TEXT,
			picture TEXT,
			profile_image_blob TEXT,
			locale TEXT,
			verified_email BOOLEAN,
			verification_code TEXT,
			type TEXT DEFAULT 'email',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			reset_token TEXT,
			reset_expires DATETIME,
			UNIQUE(email, type)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new table: %v", err)
	}

	// PROTECCIÓN CRÍTICA: Copy ALL existing data to the new table (including email users)
	log.Println("📋 Copying ALL user data to new table structure...")
	_, err = tx.Exec(`
		INSERT INTO users_new (
			id, google_id, apple_id, email, password, name, given_name, family_name,
			picture, profile_image_blob, locale, verified_email, verification_code,
			type, created_at, updated_at, reset_token, reset_expires
		)
		SELECT 
			id, 
			google_id, 
			COALESCE(apple_id, ''),
			email, 
			password, 
			name, 
			given_name, 
			family_name,
			picture, 
			profile_image_blob, 
			locale, 
			verified_email, 
			verification_code,
			COALESCE(type, 'email'),
			created_at, 
			updated_at, 
			COALESCE(reset_token, ''),
			reset_expires
		FROM users
	`)
	if err != nil {
		return fmt.Errorf("failed to copy data to new table: %v", err)
	}
	
	// VERIFICACIÓN CRÍTICA: Confirmar que todos los usuarios se copiaron
	newUserCount, err := tx.Query("SELECT COUNT(*) FROM users_new")
	if err != nil {
		log.Printf("⚠️ Warning: Could not verify new table user count: %v", err)
	} else {
		var count int
		if newUserCount.Next() {
			newUserCount.Scan(&count)
		}
		newUserCount.Close()
		log.Printf("✅ Verification: %d users copied to new table (expected: %d)", count, userCount)
		if count != userCount {
			return fmt.Errorf("CRITICAL ERROR: User count mismatch during migration! Original: %d, New: %d", userCount, count)
		}
	}

	// Drop the old table
	_, err = tx.Exec("DROP TABLE users")
	if err != nil {
		return fmt.Errorf("failed to drop old table: %v", err)
	}

	// Rename the new table
	_, err = tx.Exec("ALTER TABLE users_new RENAME TO users")
	if err != nil {
		return fmt.Errorf("failed to rename new table: %v", err)
	}

	// Recreate the apple_id index
	_, err = tx.Exec("CREATE UNIQUE INDEX idx_users_apple_id ON users(apple_id) WHERE apple_id IS NOT NULL AND apple_id != ''")
	if err != nil {
		return fmt.Errorf("failed to create apple_id index: %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration transaction: %v", err)
	}

	// VERIFICACIÓN FINAL: Confirmar que todos los usuarios están en la nueva tabla
	finalCount, err := getUserCount(db)
	if err != nil {
		log.Printf("⚠️ Warning: Could not verify final user count: %v", err)
	} else {
		log.Printf("✅ Final verification: %d users in migrated table", finalCount)
		if finalCount != userCount {
			log.Printf("🚨 WARNING: User count changed during migration! Before: %d, After: %d", userCount, finalCount)
		}
	}
	
	log.Println("✅ Table structure migration completed successfully")
	log.Printf("🔐 All %d users preserved during migration", finalCount)
	return nil
}

// getUserCount cuenta el total de usuarios en la base de datos
func getUserCount(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// getEmailTypeUsers obtiene todos los usuarios tipo email para protegerlos
func getEmailTypeUsers(db *sql.DB) ([]map[string]interface{}, error) {
	rows, err := db.Query("SELECT id, email, type FROM users WHERE type = 'email'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var emailUsers []map[string]interface{}
	for rows.Next() {
		var id int
		var email, userType string
		if err := rows.Scan(&id, &email, &userType); err != nil {
			continue
		}
		emailUsers = append(emailUsers, map[string]interface{}{
			"id": id,
			"email": email,
			"type": userType,
		})
	}
	return emailUsers, nil
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
	loadVerificationEmailTemplates()

	// Determine database path based on environment flag
	var dbPath string
	if *prodMode {
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)
	} else if *devMode {
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)
	} else {
		// Default to development mode if no flag specified
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
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

	// MIGRACIÓN DESHABILITADA PARA PREVENIR PÉRDIDA DE DATOS
	// La migración automática causaba eliminación de usuarios tipo email durante deploys
	// Check if we need to migrate from old table structure (email UNIQUE) to new structure (email, type UNIQUE)
	// err = migrateTableStructure(db)
	// if err != nil {
	//	log.Fatalf("Failed to migrate table structure: %v", err)
	// }
	log.Println("⚠️  Automatic table migration DISABLED to prevent data loss during deploys")
	log.Println("📋 Migration can be run manually if needed: migrateTableStructure()")

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

	// Check if email exists with type='email'
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND type = 'email')", req.Email).Scan(&exists)
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

	// Build the email HTML body with enhanced light theme styling that overrides device dark mode
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
        :root {
            color-scheme: light only !important;
        }

        /* Override dark mode styles */
        @media (prefers-color-scheme: dark) {
            body, html, * {
                background-color: #ffffff !important;
                color: #333333 !important;
            }

            .email-container {
                background-color: #F8E7FA !important;
                background: linear-gradient(135deg, #F8E7FA 0%%, #E6D0F0 100%%) !important;
            }

            .code-container {
                background-color: #ffffff !important;
                color: #6A1B9A !important;
            }

            .text-primary {
                color: #4A154B !important;
            }

            .text-secondary {
                color: #777777 !important;
            }
        }

        /* Ensure consistent styling across email clients */
        * {
            -webkit-text-size-adjust: 100% !important;
            -ms-text-size-adjust: 100% !important;
            -webkit-font-smoothing: antialiased !important;
            -moz-osx-font-smoothing: grayscale !important;
        }

        /* Dark mode overrides for specific email clients */
        [data-ogsc] body, [data-ogsc] * {
            background-color: #ffffff !important;
            color: #333333 !important;
        }

        [data-ogsc] .email-container {
            background-color: #F8E7FA !important;
            background: linear-gradient(135deg, #F8E7FA 0%%, #E6D0F0 100%%) !important;
        }

        [data-ogsc] .code-container {
            background-color: #ffffff !important;
            color: #6A1B9A !important;
        }
    </style>
</head>
<body style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, Arial, sans-serif !important; max-width: 600px !important; margin: 0 auto !important; padding: 20px !important; color: #333333 !important; background-color: #ffffff !important; -webkit-text-size-adjust: 100%% !important; -ms-text-size-adjust: 100%% !important;">
    <div class="email-container" style="background-color: #F8E7FA !important; background: linear-gradient(135deg, #F8E7FA 0%%, #E6D0F0 100%%) !important; border-radius: 12px !important; padding: 35px !important; text-align: center !important; box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1) !important; mso-line-height-rule: exactly !important;">
        %s
        <p class="text-primary" style="margin-bottom: 20px !important; font-size: 18px !important; color: #4A154B !important; font-weight: 500 !important; line-height: 1.4 !important; mso-line-height-rule: exactly !important;">%s</p>
        <p class="text-primary" style="margin-bottom: 30px !important; color: #4A154B !important; line-height: 1.5 !important; mso-line-height-rule: exactly !important;">%s</p>
        <p class="text-primary" style="color: #4A154B !important; font-size: 16px !important; margin-bottom: 10px !important; line-height: 1.4 !important; mso-line-height-rule: exactly !important;">%s</p>
        <div class="code-container" style="background-color: #ffffff !important; padding: 20px !important; border-radius: 8px !important; font-size: 32px !important; letter-spacing: 5px !important; font-weight: bold !important; color: #6A1B9A !important; margin: 30px auto !important; max-width: 250px !important; box-shadow: 0 3px 5px rgba(106, 27, 154, 0.2) !important; mso-line-height-rule: exactly !important; font-family: 'Courier New', Courier, monospace !important;">
            %s
        </div>
        <p class="text-primary" style="color: #4A154B !important; font-size: 14px !important; line-height: 1.4 !important; mso-line-height-rule: exactly !important;">%s</p>
    </div>
    <p class="text-secondary" style="color: #777777 !important; font-size: 12px !important; text-align: center !important; margin-top: 20px !important; line-height: 1.3 !important; mso-line-height-rule: exactly !important;">
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

	// Check if email already exists with type 'email' specifically
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND type = 'email')", req.Email).Scan(&exists)
	if err != nil {
		log.Printf("Database error checking email: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if exists {
		log.Printf("User with email %s already exists with type 'email'", req.Email)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]string{
			"error": "An account with this email already exists. Please sign in instead or use a different email address.",
		})
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
			verification_code, type
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Email, req.Password, name, givenName,
		familyName, "", processedImageBase64, req.Locale, false, // Set picture to empty for manual users, store processed image in profile_image_blob
		verificationCode, "email",
	)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		log.Printf("Request data: email=%s, name=%s", req.Email, req.Name)
		http.Error(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusInternalServerError)
		return
	}

	userID, _ := result.LastInsertId()
	log.Printf("User created with ID: %d", userID)

	// Note: Verification code Redis caching removed to improve signup speed
	// Verification codes are stored in database and checked directly

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
		SELECT id, COALESCE(google_id, '') as google_id, COALESCE(apple_id, '') as apple_id, email, name, given_name, family_name, picture, profile_image_blob, locale, verified_email, COALESCE(type, 'email') as type, created_at, updated_at 
		FROM users WHERE id = ?`, userID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.AppleID,
		&user.Email,
		&user.Name,
		&user.GivenName,
		&user.FamilyName,
		&user.Picture,
		&user.ProfileImageBlob,
		&user.Locale,
		&user.VerifiedEmail,
		&user.Type,
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

	// Basic validation: code should be exactly 6 numeric digits
	if len(code) != 6 {
		log.Printf("Invalid verification code length: %d (expected 6)", len(code))
		http.Error(w, "Invalid verification code format", http.StatusBadRequest)
		return
	}

	// Validate that code contains only digits
	for _, char := range code {
		if char < '0' || char > '9' {
			log.Printf("Invalid verification code format: contains non-numeric characters")
			http.Error(w, "Invalid verification code format", http.StatusBadRequest)
			return
		}
	}

	// Log the verification attempt with additional parameters
	log.Printf("Attempting to verify email - Code: %s, UserID: %s, Email: %s",
		code, userID, emailParam)

	// Note: Direct database verification for better performance

	// First try to find the user with the verification code
	var dbUserID int
	var email string
	var verificationCode string
	var verified bool

	err := db.QueryRow(
		"SELECT id, email, verification_code, verified_email FROM users WHERE verification_code = ? AND type = 'email'",
		code,
	).Scan(&dbUserID, &email, &verificationCode, &verified)

	// If verification code not found but user_id or email is provided,
	// try to find the user with those parameters
	if err == sql.ErrNoRows && (userID != "" || emailParam != "") {
		log.Printf("Verification code not found. Trying to find user with userID or email")

		var query string
		var queryParams []interface{}

		if userID != "" {
			query = "SELECT id, email, verification_code, verified_email FROM users WHERE id = ? AND type = 'email'"
			queryParams = []interface{}{userID}
		} else {
			query = "SELECT id, email, verification_code, verified_email FROM users WHERE email = ? AND type = 'email'"
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

			// If user is already verified, we can still return success
			if verified {
				log.Printf("User ID: %d is already verified. Returning success.", dbUserID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": "Email already verified",
					"user_id": dbUserID,
					"email":   email,
					"correct_otp": verificationCode, // Debug: Return correct OTP code
				})
				return
			}

			// Debug: Return error but include correct OTP code for testing
			log.Printf("Invalid verification code for user ID: %d", dbUserID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Invalid verification code",
				"correct_otp": verificationCode, // Debug: Return correct OTP code
				"user_id": dbUserID,
				"email": email,
			})
			return
		}
	} else if err == sql.ErrNoRows {
		log.Printf("Invalid verification code: %s - User not found", code)
		
		// Debug: Try to find any user and return their correct OTP for testing
		var debugUserID int
		var debugEmail string
		var debugOTP string
		debugErr := db.QueryRow("SELECT id, email, verification_code FROM users ORDER BY id DESC LIMIT 1").Scan(&debugUserID, &debugEmail, &debugOTP)
		if debugErr == nil {
			log.Printf("Debug: Latest user found - ID: %d, Email: %s, Correct OTP: %s", debugUserID, debugEmail, debugOTP)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Invalid verification code",
				"debug_info": map[string]interface{}{
					"latest_user_id": debugUserID,
					"latest_email": debugEmail,
					"correct_otp": debugOTP,
				},
			})
			return
		}
		
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
		"verified_otp": verificationCode, // Debug: Return the verified OTP code
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
		// If we have a user ID, use that for lookup - filter by type='email' only
		query = "SELECT id, email, name, verification_code, locale FROM users WHERE id = ? AND type = 'email'"
		queryParams = []interface{}{req.UserID}
	} else {
		// Otherwise use email - filter by type='email' only
		query = "SELECT id, email, name, verification_code, locale FROM users WHERE email = ? AND type = 'email'"
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

		// Note: Direct database storage for verification codes
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
		// If we have a user ID, use that for lookup - filter by type='email' only
		query = "SELECT verified_email FROM users WHERE id = ? AND type = 'email'"
		queryParams = []interface{}{req.UserID}
	} else {
		// Otherwise use email - filter by type='email' only
		query = "SELECT verified_email FROM users WHERE email = ? AND type = 'email'"
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

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Note: Redis verification code caching removed to improve signup performance
// All verification codes are now handled directly through the database for better speed
