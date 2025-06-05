package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nfnt/resize"
)

// Definición de estructuras de datos
type User struct {
	ID               int       `json:"id"`
	GoogleID         *string   `json:"google_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	GivenName        *string   `json:"given_name"`
	FamilyName       *string   `json:"family_name"`
	Picture          *string   `json:"picture"`
	ProfileImageBlob *string   `json:"profile_image_blob,omitempty"`
	Locale           string    `json:"locale"`
	VerifiedEmail    bool      `json:"verified_email"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	DisplayImage     string    `json:"display_image"`
}

type ProfileUpdateRequest struct {
	UserID          int    `json:"user_id"`
	Name            string `json:"name,omitempty"`
	GivenName       string `json:"given_name,omitempty"`
	FamilyName      string `json:"family_name,omitempty"`
	ProfileImageB64 string `json:"profile_image_base64,omitempty"`
}

type PasswordUpdateRequest struct {
	UserID      int    `json:"user_id"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type LocaleUpdateRequest struct {
	UserID string `json:"user_id"`
	Locale string `json:"locale"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	db *sql.DB
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

	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Database connection established successfully")
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/profile/update", corsMiddleware(handleProfileUpdate))
	http.HandleFunc("/profile/update-password", corsMiddleware(handlePasswordUpdate))
	http.HandleFunc("/profile/ping", corsMiddleware(handlePing))
	http.HandleFunc("/profile/test-image-update", corsMiddleware(handleTestImageUpdate))
	http.HandleFunc("/update/locale", corsMiddleware(handleLocaleUpdate))
	http.HandleFunc("/profile/delete-account", corsMiddleware(handleDeleteAccount))

	port := 8092 // Asignamos el puerto 8092 para el servicio de profile_management
	log.Printf("Profile Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

// Endpoint para pruebas de actualización de imagen de perfil
func handleTestImageUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Test Image Update: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("TEST IMAGE UPDATE: Recibida solicitud para usuario ID: %d", req.UserID)
	log.Printf("TEST IMAGE UPDATE: Tamaño de imagen recibida: %d bytes", len(req.ProfileImageB64))

	// Verificar que el usuario existe
	var user User
	if err := getUserById(req.UserID, &user); err != nil {
		log.Printf("TEST IMAGE UPDATE: Usuario no encontrado: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	log.Printf("TEST IMAGE UPDATE: Información de usuario antes de la actualización:")
	log.Printf("ID: %d, Email: %s, Nombre: %s", user.ID, user.Email, user.Name)
	if user.ProfileImageBlob != nil {
		log.Printf("ProfileImageBlob presente: %d bytes", len(*user.ProfileImageBlob))
	} else {
		log.Printf("ProfileImageBlob es NULL")
	}

	if user.Picture != nil {
		log.Printf("Picture presente: %s", *user.Picture)
	} else {
		log.Printf("Picture es NULL")
	}

	// Procesar la imagen
	if req.ProfileImageB64 != "" {
		processedImage, err := processImage(req.ProfileImageB64)
		if err != nil {
			log.Printf("TEST IMAGE UPDATE: Error al procesar imagen: %v", err)
			http.Error(w, fmt.Sprintf("Error processing image: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("TEST IMAGE UPDATE: Imagen procesada: %d bytes", len(processedImage))

		// Actualizar directamente la imagen de perfil en la base de datos
		result, err := db.Exec("UPDATE users SET profile_image_blob = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			processedImage, req.UserID)

		if err != nil {
			log.Printf("TEST IMAGE UPDATE: Error al actualizar imagen en la base de datos: %v", err)
			http.Error(w, "Database update failed", http.StatusInternalServerError)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("TEST IMAGE UPDATE: Actualización exitosa, filas afectadas: %d", rowsAffected)

		// Verificar el estado después de la actualización
		var updatedUser User
		if err := getUserById(req.UserID, &updatedUser); err != nil {
			log.Printf("TEST IMAGE UPDATE: Error al obtener usuario actualizado: %v", err)
		} else {
			if updatedUser.ProfileImageBlob != nil {
				log.Printf("TEST IMAGE UPDATE: Después de actualizar, ProfileImageBlob presente: %d bytes",
					len(*updatedUser.ProfileImageBlob))
			} else {
				log.Printf("TEST IMAGE UPDATE: Después de actualizar, ProfileImageBlob sigue siendo NULL")
			}
		}

		// Responder con éxito
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: true,
			Message: fmt.Sprintf("Test image update successful. Processed image size: %d bytes, Rows affected: %d",
				len(processedImage), rowsAffected),
			Data: updatedUser,
		})
		return
	}

	http.Error(w, "No image data provided", http.StatusBadRequest)
}

func handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 {
		http.Error(w, "Valid user ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating profile for user ID: %d", req.UserID)

	// Verify user exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", req.UserID).Scan(&count)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		log.Printf("User not found: %d", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Process the image if provided
	var processedImageBase64 *string
	if req.ProfileImageB64 != "" {
		log.Printf("Received image data for processing (length: %d bytes)", len(req.ProfileImageB64))
		processedImage, err := processImage(req.ProfileImageB64)
		if err != nil {
			log.Printf("Failed to process image: %v", err)
			// Continue without updating the image
		} else {
			processedImageBase64 = &processedImage
			log.Printf("Successfully processed and compressed profile image (processed length: %d bytes)", len(processedImage))

			// Verificamos que la imagen procesada no esté vacía
			if len(processedImage) == 0 {
				log.Printf("ERROR: La imagen procesada está vacía")
				processedImageBase64 = nil
			}
		}
	}

	// Build the update query dynamically based on provided fields
	updateQuery := "UPDATE users SET updated_at = CURRENT_TIMESTAMP"
	var params []interface{}

	if req.Name != "" {
		updateQuery += ", name = ?"
		params = append(params, req.Name)
	}

	if req.GivenName != "" {
		updateQuery += ", given_name = ?"
		params = append(params, req.GivenName)
	}

	if req.FamilyName != "" {
		updateQuery += ", family_name = ?"
		params = append(params, req.FamilyName)
	}

	if processedImageBase64 != nil {
		updateQuery += ", profile_image_blob = ?"
		params = append(params, *processedImageBase64)
	}

	if len(params) == 0 {
		// No fields to update
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: false,
			Message: "No fields to update",
		})
		return
	}

	// Add the WHERE clause and user ID parameter
	updateQuery += " WHERE id = ?"
	params = append(params, req.UserID)

	// Log debug information
	log.Printf("Update query: %s", updateQuery)
	log.Printf("Update params count: %d", len(params))

	// Debug log for image parameter
	for i, p := range params {
		if s, ok := p.(string); ok && len(s) > 100 {
			log.Printf("Param %d: %s... (length: %d)", i, s[:min(100, len(s))], len(s))
		} else if s, ok := p.(string); ok {
			log.Printf("Param %d: %s (length: %d)", i, s, len(s))
		} else {
			log.Printf("Param %d: %v", i, p)
		}
	}

	// Execute the update directly instead of using a prepared statement
	result, err := db.Exec(updateQuery, params...)
	if err != nil {
		log.Printf("Failed to execute update: %v", err)

		// Intentar una actualización separada solo para la imagen si es lo que falló
		if processedImageBase64 != nil {
			log.Printf("Attempting separate image update")

			// Intentar una actualización directa solo de la imagen
			imgResult, imgErr := db.Exec(
				"UPDATE users SET profile_image_blob = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
				*processedImageBase64, req.UserID,
			)

			if imgErr != nil {
				log.Printf("Separate image update also failed: %v", imgErr)
			} else {
				imgRows, _ := imgResult.RowsAffected()
				log.Printf("Separate image update succeeded! Rows affected: %d", imgRows)

				// Si la actualización de la imagen fue exitosa, actualizar el resto de los campos si es necesario
				if req.Name != "" || req.GivenName != "" || req.FamilyName != "" {
					otherFieldsQuery := "UPDATE users SET updated_at = CURRENT_TIMESTAMP"
					var otherParams []interface{}

					if req.Name != "" {
						otherFieldsQuery += ", name = ?"
						otherParams = append(otherParams, req.Name)
					}

					if req.GivenName != "" {
						otherFieldsQuery += ", given_name = ?"
						otherParams = append(otherParams, req.GivenName)
					}

					if req.FamilyName != "" {
						otherFieldsQuery += ", family_name = ?"
						otherParams = append(otherParams, req.FamilyName)
					}

					otherFieldsQuery += " WHERE id = ?"
					otherParams = append(otherParams, req.UserID)

					_, otherErr := db.Exec(otherFieldsQuery, otherParams...)
					if otherErr != nil {
						log.Printf("Error updating other fields: %v", otherErr)
					} else {
						log.Printf("Other fields updated successfully")
					}
				}

				// Get the updated user
				var user User
				err = getUserById(req.UserID, &user)
				if err == nil {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(ApiResponse{
						Success: true,
						Message: "Profile partially updated successfully (only image)",
						Data:    user,
					})
					return
				}
			}
		}

		http.Error(w, "Failed to update user profile", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Successfully updated user %d, %d rows affected", req.UserID, rowsAffected)

	// Get the updated user to return in the response
	var user User
	err = getUserById(req.UserID, &user)
	if err != nil {
		log.Printf("Error retrieving updated user: %v", err)
		// Still return success even if we can't fetch the updated user
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: true,
			Message: "Profile updated successfully",
		})
		return
	}

	// Return the updated user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: "Profile updated successfully",
		Data:    user,
	})
}

func handlePasswordUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PasswordUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 || req.OldPassword == "" || req.NewPassword == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating password for user ID: %d", req.UserID)

	// Verify old password
	var currentPassword string
	err := db.QueryRow("SELECT password FROM users WHERE id = ?", req.UserID).Scan(&currentPassword)

	if err == sql.ErrNoRows {
		log.Printf("User not found: %d", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Verify old password matches
	// In a real app, you would compare hashed passwords
	if currentPassword != req.OldPassword {
		log.Printf("Incorrect password for user ID: %d", req.UserID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: false,
			Message: "Current password is incorrect",
		})
		return
	}

	// Update password
	_, err = db.Exec("UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.NewPassword, req.UserID)

	if err != nil {
		log.Printf("Failed to update password: %v", err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	log.Printf("Password updated successfully for user ID: %d", req.UserID)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: "Password updated successfully",
	})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Profile Management service is running",
	})
}

func handleLocaleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LocaleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Locale == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating locale for user ID: %s", req.UserID)

	// Verify user exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", req.UserID).Scan(&count)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		log.Printf("User not found: %s", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Update locale
	_, err = db.Exec("UPDATE users SET locale = ? WHERE id = ?", req.Locale, req.UserID)
	if err != nil {
		log.Printf("Failed to update locale: %v", err)
		http.Error(w, "Failed to update locale", http.StatusInternalServerError)
		return
	}

	log.Printf("Locale updated successfully for user ID: %s", req.UserID)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := ApiResponse{
		Success: true,
		Message: "Locale updated successfully",
		Data:    req,
	}
	json.NewEncoder(w).Encode(response)
}

// DeleteAccountRequest structure for handling account deletion requests
type DeleteAccountRequest struct {
	UserID int `json:"user_id"`
}

// handleDeleteAccount handles the complete deletion of user account and all associated data
func handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Delete Account: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("DELETE ACCOUNT: Iniciando eliminación completa para usuario ID: %d", req.UserID)

	// Verificar que el usuario existe antes de eliminarlo
	var user User
	if err := getUserById(req.UserID, &user); err != nil {
		log.Printf("DELETE ACCOUNT: Usuario no encontrado: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	log.Printf("DELETE ACCOUNT: Usuario encontrado: %s (%d)", user.Email, user.ID)

	// Iniciar transacción para garantizar atomicidad
	tx, err := db.Begin()
	if err != nil {
		log.Printf("DELETE ACCOUNT: Error iniciando transacción: %v", err)
		http.Error(w, "Database transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Lista de tablas a limpiar (orden importante por las foreign keys)
	tables := []string{
		"categories",
		"cash_bank_transactions",
		"cash_bank",
		"daily_balance",
		"weekly_balance",
		"monthly_balance",
		"daily_cash_bank_balance",
		"weekly_cash_bank_balance",
		"monthly_cash_bank_balance",
		"bills",
		"expenses",
		"incomes",
		"savings",
		"balances",
		"users",
	}

	userIDStr := fmt.Sprintf("%d", req.UserID)

	// Eliminar registros de todas las tablas
	for _, table := range tables {
		var query string

		// Para la tabla users, usar 'id' como campo
		if table == "users" {
			query = fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
		} else {
			// Para el resto de tablas, usar 'user_id' como campo
			query = fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", table)
		}

		result, err := tx.Exec(query, userIDStr)
		if err != nil {
			log.Printf("DELETE ACCOUNT: Error eliminando de tabla %s: %v", table, err)
			http.Error(w, fmt.Sprintf("Failed to delete from %s", table), http.StatusInternalServerError)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("DELETE ACCOUNT: Eliminados %d registros de tabla %s", rowsAffected, table)
	}

	// Confirmar la transacción
	if err := tx.Commit(); err != nil {
		log.Printf("DELETE ACCOUNT: Error confirmando transacción: %v", err)
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("DELETE ACCOUNT: Eliminación completa exitosa para usuario %s (%d)", user.Email, user.ID)

	// Responder con éxito
	w.Header().Set("Content-Type", "application/json")
	response := ApiResponse{
		Success: true,
		Message: fmt.Sprintf("Cuenta y todos los datos del usuario %s eliminados exitosamente", user.Email),
	}
	json.NewEncoder(w).Encode(response)
}

func getUserById(userID int, user *User) error {
	err := db.QueryRow(`
		SELECT id, google_id, email, name, given_name, family_name, 
		picture, profile_image_blob, locale, verified_email, created_at, updated_at 
		FROM users 
		WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.GoogleID,
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
		return err
	}

	// Set the display image based on the user type
	if user.GoogleID != nil && *user.GoogleID != "" {
		// Google user - use Picture URL field
		if user.Picture != nil && *user.Picture != "" {
			user.DisplayImage = *user.Picture
		}
	} else {
		// Regular user - use ProfileImageBlob field
		if user.ProfileImageBlob != nil && *user.ProfileImageBlob != "" {
			user.DisplayImage = *user.ProfileImageBlob
		}
	}

	return nil
}

// Process image: resize, compress, and convert to WebP
func processImage(base64Image string) (string, error) {
	log.Printf("Processing image, input length: %d", len(base64Image))

	// Extract the actual base64 content from the data URL
	base64Data := base64Image
	if idx := strings.Index(base64Image, ";base64,"); idx > 0 {
		base64Data = base64Image[idx+8:]
		log.Printf("Extracted base64 data after prefix, new length: %d", len(base64Data))
	}

	// Check if the base64 string is valid
	if len(base64Data) == 0 {
		return "", fmt.Errorf("empty base64 image data")
	}

	// Clean the base64 string - remove any whitespace or newlines
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	base64Data = strings.ReplaceAll(base64Data, " ", "")

	// Ensure padding is correct (base64 string length must be multiple of 4)
	for len(base64Data)%4 != 0 {
		base64Data += "="
	}

	log.Printf("Cleaned base64 data, final length: %d", len(base64Data))

	// Decode base64 image
	imgData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		log.Printf("Base64 decode error: %v", err)
		// If standard decoding fails, try URL-safe decoding
		imgData, err = base64.URLEncoding.DecodeString(base64Data)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 image (tried standard and URL-safe): %v", err)
		}
		log.Printf("Successfully decoded using URL-safe base64")
	} else {
		log.Printf("Successfully decoded using standard base64")
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

	// Instead of WebP (which might have compatibility issues), use standard JPEG for better compatibility
	var jpegBuf bytes.Buffer
	err = jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		log.Printf("Failed to encode JPEG: %v, falling back to PNG", err)
		// If JPEG encoding fails, try PNG as fallback
		var pngBuf bytes.Buffer
		err = png.Encode(&pngBuf, img)
		if err != nil {
			return "", fmt.Errorf("failed to encode both JPEG and PNG: %v", err)
		}
		log.Printf("Successfully encoded as PNG, size: %d KB", pngBuf.Len()/1024)
		// Convert back to base64
		encodedImage := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
		log.Printf("Final encoded PNG image size: %d bytes", len(encodedImage))
		return encodedImage, nil
	}

	// Check if the compressed image is still too large (>100KB)
	compressedSize := jpegBuf.Len()
	log.Printf("Compressed JPEG size: %d KB", compressedSize/1024)

	// If still too large, compress more
	if compressedSize > 100*1024 {
		jpegBuf.Reset()
		quality := 70
		for compressedSize > 100*1024 && quality > 30 {
			jpegBuf.Reset()
			err = jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: quality})
			if err != nil {
				return "", fmt.Errorf("failed to encode JPEG with quality %d: %v", quality, err)
			}
			compressedSize = jpegBuf.Len()
			quality -= 10
			log.Printf("Recompressed JPEG size: %d KB (quality: %d)", compressedSize/1024, quality)
		}
	}

	// Convert back to base64 with JPEG prefix
	encodedImage := base64.StdEncoding.EncodeToString(jpegBuf.Bytes())
	log.Printf("Final encoded JPEG image size: %d bytes", len(encodedImage))
	return encodedImage, nil
}

// Función auxiliar para encontrar el mínimo de dos enteros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
