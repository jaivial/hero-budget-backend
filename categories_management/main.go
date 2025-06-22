package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	_ "github.com/mattn/go-sqlite3"
)

// Definición de estructuras de datos
type Category struct {
	ID        int    `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "income" o "expense"
	Emoji     string `json:"emoji"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type AddCategoryRequest struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Type   string `json:"type"` // "income" o "expense"
	Emoji  string `json:"emoji"`
}

type UpdateCategoryRequest struct {
	UserID     string `json:"user_id"`
	CategoryID int    `json:"category_id"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"` // "income" o "expense"
	Emoji      string `json:"emoji,omitempty"`
}

type DeleteCategoryRequest struct {
	UserID     string `json:"user_id"`
	CategoryID int    `json:"category_id"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	// Database connection for category data persistence
	db *sql.DB
	// Redis client for caching category data
	rdb *redis.Client
	// Context for Redis operations with timeout handling
	ctx = context.Background()
)

func init() {
	var err error

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection with UTF-8 encoding support
	db, err = openDatabaseConnection()
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	log.Println("Database connection established successfully")

	// Initialize Redis connection for category data caching
	initRedis()
}

// initRedis initializes Redis connection for category data caching
// Provides distributed caching for category lists and frequent queries
func initRedis() {
	// Redis connection configuration with environment variable support
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address for local development
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 9 // Use DB 9 for categories_management service to avoid conflicts

	// Create Redis client with connection pooling and timeout settings
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DB:           redisDB,
		PoolSize:     10,                // Connection pool size for concurrent requests
		DialTimeout:  5 * time.Second,   // Connection establishment timeout
		ReadTimeout:  3 * time.Second,   // Read operation timeout
		WriteTimeout: 3 * time.Second,   // Write operation timeout
		ConnMaxIdleTime: 5 * time.Minute, // Idle connection timeout
	})

	// Test Redis connection with ping operation
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Redis connection failed (continuing without cache): %v", err)
		rdb = nil // Disable Redis if connection fails
		return
	}

	log.Println("Redis connection established successfully for category caching")
}

// getCategoriesFromCache retrieves cached category data for a user
// Returns cached categories if available, reducing database load for frequent requests
func getCategoriesFromCache(userID string) ([]Category, error) {
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	// Generate cache key with service namespace for data isolation
	cacheKey := fmt.Sprintf("categories_management:categories:%s", userID)
	
	// Retrieve JSON data from Redis cache
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		log.Printf("Cache miss for categories: %s", userID)
		return nil, fmt.Errorf("cache miss")
	} else if err != nil {
		log.Printf("Redis error retrieving categories cache for %s: %v", userID, err)
		return nil, err
	}

	// Deserialize JSON data to Category slice
	var categories []Category
	err = json.Unmarshal([]byte(val), &categories)
	if err != nil {
		log.Printf("Error deserializing cached categories for %s: %v", userID, err)
		return nil, err
	}

	log.Printf("Cache hit for categories: %s", userID)
	return categories, nil
}

// cacheCategories stores category data in Redis cache with TTL
// Implements 30-minute TTL for category data as it changes less frequently
func cacheCategories(userID string, categories []Category) {
	if rdb == nil {
		return
	}

	// Generate cache key with service namespace
	cacheKey := fmt.Sprintf("categories_management:categories:%s", userID)
	
	// Serialize category data to JSON for storage
	categoriesData, err := json.Marshal(categories)
	if err != nil {
		log.Printf("Error serializing categories for cache: %v", err)
		return
	}

	// Store in Redis with 30-minute TTL (1800 seconds)
	// Categories change less frequently and can be cached for longer periods
	err = rdb.Set(ctx, cacheKey, categoriesData, 30*time.Minute).Err()
	if err != nil {
		log.Printf("Error caching categories for %s: %v", userID, err)
		return
	}

	log.Printf("Categories cached successfully for: %s", userID)
}

// invalidateCategoriesCache removes cached category data when categories are modified
// Ensures cache consistency after category CRUD operations
func invalidateCategoriesCache(userID string) {
	if rdb == nil {
		return
	}

	// Generate cache key for deletion
	cacheKey := fmt.Sprintf("categories_management:categories:%s", userID)
	
	// Remove cached data to force fresh fetch on next request
	err := rdb.Del(ctx, cacheKey).Err()
	if err != nil {
		log.Printf("Error invalidating categories cache for %s: %v", userID, err)
	} else {
		log.Printf("Categories cache invalidated for: %s", userID)
	}
}

func openDatabaseConnection() (*sql.DB, error) {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join("..", "google_auth", "users.db")
	fullDbPath := filepath.Join(cwd, dbPath)

	// Asegurar que la ruta existe
	dbFolder := filepath.Dir(fullDbPath)
	if _, err := os.Stat(dbFolder); os.IsNotExist(err) {
		if err := os.MkdirAll(dbFolder, os.ModePerm); err != nil {
			return nil, err
		}
	}

	// Abrir la base de datos con parámetros específicos para UTF-8
	db, err := sql.Open("sqlite3", fullDbPath+"?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_case_sensitive_like=off&_encoding=UTF-8")
	if err != nil {
		return nil, err
	}

	// Verificar la conexión
	if err = db.Ping(); err != nil {
		return nil, err
	}

	// Configurar PRAGMA para UTF-8
	if _, err := db.Exec("PRAGMA encoding = 'UTF-8';"); err != nil {
		log.Printf("Error al configurar PRAGMA encoding UTF-8: %v", err)
		// Continuar a pesar del error
	}

	// Crear la tabla si no existe
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		emoji TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/categories", corsMiddleware(handleFetchCategories))
	http.HandleFunc("/categories/add", corsMiddleware(handleAddCategory))
	http.HandleFunc("/categories/update", corsMiddleware(handleUpdateCategory))
	http.HandleFunc("/categories/delete", corsMiddleware(handleDeleteCategory))
	http.HandleFunc("/categories/fix-emojis", corsMiddleware(handleFixEmojis))

	port := 8096 // Puerto para el servicio de categorías
	log.Printf("Categories Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it's OPTIONS, return with just the headers (preflight request)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}

func handleFetchCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get optional type filter
	categoryType := r.URL.Query().Get("type") // "income", "expense", or empty for all

	// Get categories from database
	categories, err := fetchCategories(userID, categoryType)
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		sendErrorResponse(w, "Error fetching categories", http.StatusInternalServerError)
		return
	}

	// Return categories as JSON
	sendSuccessResponse(w, "Categories fetched successfully", categories)
}

func handleAddCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var addRequest AddCategoryRequest
	err := json.NewDecoder(r.Body).Decode(&addRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if addRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if addRequest.Name == "" {
		sendErrorResponse(w, "Category name is required", http.StatusBadRequest)
		return
	}

	if addRequest.Type != "income" && addRequest.Type != "expense" {
		sendErrorResponse(w, "Category type must be 'income' or 'expense'", http.StatusBadRequest)
		return
	}

	// Set default emoji if not provided
	if addRequest.Emoji == "" {
		if addRequest.Type == "income" {
			addRequest.Emoji = "💰"
		} else {
			addRequest.Emoji = "🛒"
		}
	}

	// Create category object
	category := Category{
		UserID: addRequest.UserID,
		Name:   addRequest.Name,
		Type:   addRequest.Type,
		Emoji:  addRequest.Emoji,
	}

	// Add category to database
	categoryID, err := addCategory(category)
	if err != nil {
		log.Printf("Error adding category: %v", err)
		sendErrorResponse(w, "Error adding category", http.StatusInternalServerError)
		return
	}

	// Recuperar la categoría recién creada para asegurar que el emoji está correcto
	createdCategory, err := fetchCategoryByID(categoryID, addRequest.UserID)
	if err != nil {
		log.Printf("Error fetching created category: %v", err)
		sendErrorResponse(w, "Error fetching created category", http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG - Emoji después de creación: %s", createdCategory.Emoji)

	// Return success response with the created category
	sendSuccessResponse(w, "Category added successfully", createdCategory)
}

func handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest UpdateCategoryRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if updateRequest.CategoryID <= 0 {
		sendErrorResponse(w, "Valid category ID is required", http.StatusBadRequest)
		return
	}

	if updateRequest.Type != "" && updateRequest.Type != "income" && updateRequest.Type != "expense" {
		sendErrorResponse(w, "Category type must be 'income' or 'expense'", http.StatusBadRequest)
		return
	}

	// Fetch existing category
	existingCategory, err := fetchCategoryByID(updateRequest.CategoryID, updateRequest.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(w, "Category not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching category: %v", err)
			sendErrorResponse(w, "Error fetching category", http.StatusInternalServerError)
		}
		return
	}

	// Update fields if provided
	if updateRequest.Name != "" {
		existingCategory.Name = updateRequest.Name
	}
	if updateRequest.Type != "" {
		existingCategory.Type = updateRequest.Type
	}
	if updateRequest.Emoji != "" {
		existingCategory.Emoji = updateRequest.Emoji
	}

	// Update category in database
	err = updateCategory(*existingCategory)
	if err != nil {
		log.Printf("Error updating category: %v", err)
		sendErrorResponse(w, "Error updating category", http.StatusInternalServerError)
		return
	}

	// IMPORTANTE: Recuperar la categoría actualizada para obtener la información correcta
	updatedCategory, err := fetchCategoryByID(updateRequest.CategoryID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching updated category: %v", err)
		sendErrorResponse(w, "Error fetching updated category", http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG - Emoji después de actualización: %s", updatedCategory.Emoji)

	// Return success response with the updated category
	sendSuccessResponse(w, "Category updated successfully", updatedCategory)
}

func handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var deleteRequest DeleteCategoryRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if deleteRequest.CategoryID <= 0 {
		sendErrorResponse(w, "Valid category ID is required", http.StatusBadRequest)
		return
	}

	// Delete category from database
	err = deleteCategory(deleteRequest.CategoryID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting category: %v", err)
		sendErrorResponse(w, "Error deleting category", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Category deleted successfully", nil)
}

func handleFixEmojis(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Iniciando corrección de emojis corruptos...")

	// Get all categories with corrupted emojis and those that are not properly encoded
	rows, err := db.Query(`SELECT id, user_id, emoji FROM categories WHERE emoji = 'ð' OR emoji = 'ð ' OR emoji LIKE '%ð%' OR emoji NOT LIKE 'BASE64:%'`)
	if err != nil {
		log.Printf("Error al consultar categorías: %v", err)
		sendErrorResponse(w, "Error al consultar categorías", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Contador para categorías actualizadas
	var updatedCount int
	var updatedCategories []Category

	// Mapa de emojis predeterminados basados en el ID de categoría (para tener variedad)
	defaultEmojis := []string{
		"📊", "💰", "🛒", "🏠", "🚗", "✈️", "🍔", "🍕", "💼", "💸", "💳", "💵",
	}

	// Procesar cada categoría
	for rows.Next() {
		var id int
		var userId string
		var emoji string

		if err := rows.Scan(&id, &userId, &emoji); err != nil {
			log.Printf("Error al escanear fila: %v", err)
			continue
		}

		// Elegir un emoji predeterminado basado en el ID (para variar)
		defaultEmoji := defaultEmojis[id%len(defaultEmojis)]

		// Codificar el emoji en base64
		encodedEmoji := encodeEmoji(defaultEmoji)

		// Actualizar la categoría
		_, err := db.Exec(
			`UPDATE categories SET emoji = ? WHERE id = ? AND user_id = ?`,
			encodedEmoji, id, userId,
		)

		if err != nil {
			log.Printf("Error al actualizar categoría %d: %v", id, err)
			continue
		}

		// Obtener la categoría actualizada
		updatedCategory, err := fetchCategoryByID(id, userId)
		if err != nil {
			log.Printf("Error al obtener categoría actualizada %d: %v", id, err)
			continue
		}

		updatedCategories = append(updatedCategories, *updatedCategory)
		updatedCount++
		log.Printf("Categoría %d actualizada con éxito: %s -> %s", id, emoji, updatedCategory.Emoji)
	}

	log.Printf("Proceso completado. %d categorías actualizadas.", updatedCount)
	sendSuccessResponse(w, fmt.Sprintf("%d categorías actualizadas", updatedCount), updatedCategories)
}

// Database functions
func fetchCategories(userID, categoryType string) ([]Category, error) {
	var query string
	var args []interface{}

	if categoryType == "" {
		// Fetch all categories for the user
		query = `SELECT id, user_id, name, type, emoji, created_at, updated_at FROM categories WHERE user_id = ? ORDER BY name ASC`
		args = []interface{}{userID}
	} else {
		// Fetch categories of specific type
		query = `SELECT id, user_id, name, type, emoji, created_at, updated_at FROM categories WHERE user_id = ? AND type = ? ORDER BY name ASC`
		args = []interface{}{userID, categoryType}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var category Category
		var encodedEmoji string

		err := rows.Scan(
			&category.ID,
			&category.UserID,
			&category.Name,
			&category.Type,
			&encodedEmoji, // Leer el emoji codificado
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Decodificar el emoji antes de agregarlo al objeto Category
		category.Emoji = decodeEmoji(encodedEmoji)

		categories = append(categories, category)
	}

	return categories, nil
}

func fetchCategoryByID(categoryID int, userID string) (*Category, error) {
	var category Category
	var encodedEmoji string

	err := db.QueryRow(
		`SELECT id, user_id, name, type, emoji, created_at, updated_at FROM categories WHERE id = ? AND user_id = ?`,
		categoryID, userID,
	).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Type,
		&encodedEmoji,
		&category.CreatedAt,
		&category.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Decodificar el emoji
	category.Emoji = decodeEmoji(encodedEmoji)

	return &category, nil
}

func addCategory(category Category) (int, error) {
	// Codificar el emoji antes de guardarlo
	encodedEmoji := encodeEmoji(category.Emoji)

	result, err := db.Exec(
		`INSERT INTO categories (user_id, name, type, emoji) VALUES (?, ?, ?, ?)`,
		category.UserID, category.Name, category.Type, encodedEmoji,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func updateCategory(category Category) error {
	// Codificar el emoji antes de guardarlo
	encodedEmoji := encodeEmoji(category.Emoji)

	_, err := db.Exec(
		`UPDATE categories SET name = ?, type = ?, emoji = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`,
		category.Name, category.Type, encodedEmoji, category.ID, category.UserID,
	)
	return err
}

func deleteCategory(categoryID int, userID string) error {
	_, err := db.Exec(
		`DELETE FROM categories WHERE id = ? AND user_id = ?`,
		categoryID, userID,
	)
	return err
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
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

// Función para codificar emojis como Base64 antes de almacenarlos
func encodeEmoji(emoji string) string {
	if emoji == "" {
		return "📊" // Emoji predeterminado
	}

	// Verificar si el emoji necesita ser codificado (caracteres no ASCII)
	needsEncoding := false
	for _, r := range emoji {
		if r > 127 { // Caracteres fuera del rango ASCII
			needsEncoding = true
			break
		}
	}

	if needsEncoding {
		// Convertir a bytes UTF-8 y luego a base64
		encoded := base64.StdEncoding.EncodeToString([]byte(emoji))
		result := "BASE64:" + encoded
		log.Printf("DEBUG - Emoji codificado: '%s' -> '%s'", emoji, result)
		return result
	}

	return emoji
}

// Función para decodificar emojis de Base64 cuando se recuperan
func decodeEmoji(encoded string) string {
	// Si está vacío o es el emoji predeterminado, devolverlo tal cual
	if encoded == "" || encoded == "📊" {
		return encoded
	}

	// Si está codificado en Base64, decodificarlo
	if strings.HasPrefix(encoded, "BASE64:") {
		encodedPart := strings.TrimPrefix(encoded, "BASE64:")
		decoded, err := base64.StdEncoding.DecodeString(encodedPart)
		if err != nil {
			log.Printf("ERROR - Error al decodificar emoji: %v", err)
			return "📊" // En caso de error, devolver el emoji predeterminado
		}

		// Verificar que la decodificación resultó en UTF-8 válido
		decodedStr := string(decoded)
		if !utf8.ValidString(decodedStr) {
			log.Printf("ERROR - El emoji decodificado no es UTF-8 válido")
			return "📊"
		}

		log.Printf("DEBUG - Emoji decodificado: '%s' -> '%s'", encoded, decodedStr)
		return decodedStr
	}

	// Verificar caracteres corruptos comunes
	if encoded == "ð" || encoded == "ð " || strings.Contains(encoded, "ð") || strings.Contains(encoded, "â") {
		log.Printf("DEBUG - Emoji corrupto detectado: '%s', usando predeterminado", encoded)
		return "📊"
	}

	// Verificar que el string sea UTF-8 válido
	if !utf8.ValidString(encoded) {
		log.Printf("ERROR - El emoji no es UTF-8 válido: '%s'", encoded)
		return "📊"
	}

	return encoded
}
