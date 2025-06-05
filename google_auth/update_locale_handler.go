package main

import (
	"encoding/json"
	"log"
	"net/http"
)

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
