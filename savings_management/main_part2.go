package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Funciones de base de datos y handlers complementarios para savings_management
// Contiene operaciones CRUD, validaciones y handlers de endpoints restantes

// handleUpdateSavings maneja las solicitudes de actualización de ahorros
// Endpoint: POST /savings/update
// Actualiza los datos de ahorro para un usuario específico con cálculos automáticos
func handleUpdateSavings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body with error handling
	var updateRequest SavingsUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request - user ID is mandatory
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get current savings data to update only the fields that were provided
	// Esto permite actualizaciones parciales sin sobrescribir datos existentes
	currentSavings, err := fetchSavingsData(updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current savings data: %v", err)
		sendErrorResponse(w, "Error fetching current savings data", http.StatusInternalServerError)
		return
	}

	// Update only the fields that were provided (partial update logic)
	if updateRequest.Available > 0 {
		currentSavings.Available = updateRequest.Available
	}
	if updateRequest.Goal > 0 {
		currentSavings.Goal = updateRequest.Goal
	}
	if updateRequest.Period != "" {
		currentSavings.Period = updateRequest.Period
	}

	// Calculate the percentage of goal achievement
	if currentSavings.Goal > 0 {
		currentSavings.Percent = (currentSavings.Available / currentSavings.Goal) * 100
	} else {
		currentSavings.Percent = 0
	}

	// Calculate need to save and daily target for goal tracking
	currentSavings.NeedToSave = currentSavings.Goal - currentSavings.Available
	if currentSavings.NeedToSave < 0 {
		currentSavings.NeedToSave = 0
	}
	// Assuming goal needs to be achieved within a month (30 days)
	currentSavings.DailyTarget = currentSavings.NeedToSave / 30

	// Save the updated savings data to database
	err = updateSavingsData(currentSavings)
	if err != nil {
		log.Printf("Error updating savings data: %v", err)
		sendErrorResponse(w, "Error updating savings data", http.StatusInternalServerError)
		return
	}

	// Record sync operation if sync parameters are provided
	if updateRequest.OperationID != "" && updateRequest.DeviceID != "" && updateRequest.Timestamp > 0 {
		log.Printf("Recording sync operation for savings update: operation_id=%s, device_id=%s, timestamp=%d", 
			updateRequest.OperationID, updateRequest.DeviceID, updateRequest.Timestamp)
		
		// Create sync operation data for savings update
		syncData := map[string]interface{}{
			"user_id":   updateRequest.UserID,
			"available": currentSavings.Available,
			"goal":      currentSavings.Goal,
			"period":    currentSavings.Period,
			"percent":   currentSavings.Percent,
			"action":    "update_savings",
		}
		
		// Add sync operation record to database
		err = addSyncOperation(
			updateRequest.UserID,
			updateRequest.OperationID,
			"update",
			"savings",
			fmt.Sprintf("%s", updateRequest.UserID),
			syncData,
			updateRequest.DeviceID,
			updateRequest.Timestamp,
		)
		
		if err != nil {
			log.Printf("Warning: Failed to record sync operation for savings update: %v", err)
			// Don't fail the savings update for sync errors, just log warning
		} else {
			log.Printf("Successfully recorded sync operation for savings update: user=%s", updateRequest.UserID)
		}
	} else {
		log.Printf("Sync parameters not provided or incomplete, skipping sync operation recording")
	}

	// Invalidate cache since savings data was updated
	// Cache invalidation asegura consistencia de datos
	if cacheManager != nil {
		err = cacheManager.InvalidateSavingsCache(updateRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate savings cache for user %s: %v", updateRequest.UserID, err)
		}
		
		// Also invalidate dashboard cache since savings affect dashboard
		err = cacheManager.InvalidateDashboardCache(updateRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", updateRequest.UserID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (savings and dashboard)", updateRequest.UserID)
	}

	// Return success response with updated data
	sendSuccessResponse(w, "Savings updated successfully", currentSavings)
}

// handleDeleteSavings maneja las solicitudes de eliminación de metas de ahorro
// Endpoint: DELETE /savings/delete
// Elimina completamente los datos de ahorro de un usuario específico
func handleDeleteSavings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for user identification
	var deleteRequest SavingsDeleteRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request - user ID is required for deletion
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Delete the savings data from database
	err = deleteSavingsData(deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting savings data: %v", err)
		sendErrorResponse(w, "Error deleting savings data", http.StatusInternalServerError)
		return
	}

	// Record sync operation if sync parameters are provided
	if deleteRequest.OperationID != "" && deleteRequest.DeviceID != "" && deleteRequest.Timestamp > 0 {
		log.Printf("Recording sync operation for savings delete: operation_id=%s, device_id=%s, timestamp=%d", 
			deleteRequest.OperationID, deleteRequest.DeviceID, deleteRequest.Timestamp)
		
		// Create sync operation data for savings delete
		syncData := map[string]interface{}{
			"user_id": deleteRequest.UserID,
			"action":  "delete_savings",
		}
		
		// Add sync operation record to database
		err = addSyncOperation(
			deleteRequest.UserID,
			deleteRequest.OperationID,
			"delete",
			"savings",
			fmt.Sprintf("%s", deleteRequest.UserID),
			syncData,
			deleteRequest.DeviceID,
			deleteRequest.Timestamp,
		)
		
		if err != nil {
			log.Printf("Warning: Failed to record sync operation for savings delete: %v", err)
			// Don't fail the savings delete for sync errors, just log warning
		} else {
			log.Printf("Successfully recorded sync operation for savings delete: user=%s", deleteRequest.UserID)
		}
	} else {
		log.Printf("Sync parameters not provided or incomplete, skipping sync operation recording")
	}

	// Invalidate cache since savings data was deleted
	// Cache invalidation es crítica después de eliminaciones
	if cacheManager != nil {
		err = cacheManager.InvalidateSavingsCache(deleteRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate savings cache for user %s: %v", deleteRequest.UserID, err)
		}
		
		// Also invalidate dashboard cache since savings affect dashboard
		err = cacheManager.InvalidateDashboardCache(deleteRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", deleteRequest.UserID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (savings and dashboard)", deleteRequest.UserID)
	}

	// Return success response confirming deletion
	sendSuccessResponse(w, "Savings goal deleted successfully", nil)
}

// fetchSavingsData obtiene los datos de ahorro de un usuario desde la base de datos
// Retorna datos por defecto si no existe información previa para el usuario
func fetchSavingsData(userID string) (SavingsData, error) {
	var savings SavingsData

	// Query savings data from database with comprehensive field selection
	err := db.QueryRow(`
		SELECT user_id, available, goal, period, percent
		FROM savings
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&savings.UserID,
		&savings.Available,
		&savings.Goal,
		&savings.Period,
		&savings.Percent,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found for the user
		// Esto permite que nuevos usuarios tengan una estructura base consistente
		savings.UserID = userID
		savings.Available = 0
		savings.Goal = 0
		savings.Period = "monthly" // Default period
		savings.Percent = 0
		savings.NeedToSave = 0
		savings.DailyTarget = 0
		return savings, nil
	} else if err != nil {
		return savings, err
	}

	// Calculate derived fields for goal tracking and progress visualization
	savings.NeedToSave = savings.Goal - savings.Available
	if savings.NeedToSave < 0 {
		savings.NeedToSave = 0
	}
	// Calculate daily target assuming goal completion within a month (30 days)
	savings.DailyTarget = savings.NeedToSave / 30

	return savings, nil
}

// updateSavingsData actualiza o inserta datos de ahorro en la base de datos
// Utiliza lógica upsert para manejar tanto actualizaciones como nuevas entradas
func updateSavingsData(savings SavingsData) error {
	// Check if a savings entry already exists for this user
	// Esto determina si debemos hacer UPDATE o INSERT
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM savings 
		WHERE user_id = ?
	`, savings.UserID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing savings entry with all relevant fields
		_, err = db.Exec(`
			UPDATE savings
			SET available = ?,
				goal = ?,
				period = ?,
				percent = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?
		`,
			savings.Available,
			savings.Goal,
			savings.Period,
			savings.Percent,
			savings.UserID,
		)
	} else {
		// Insert new savings entry with complete data structure
		_, err = db.Exec(`
			INSERT INTO savings (
				user_id, available, goal, period, percent
			) VALUES (?, ?, ?, ?, ?)
		`,
			savings.UserID,
			savings.Available,
			savings.Goal,
			savings.Period,
			savings.Percent,
		)
	}

	return err
}

// deleteSavingsData elimina los datos de ahorro de un usuario específico
// Incluye validación para confirmar que la eliminación fue exitosa
func deleteSavingsData(userID string) error {
	// Execute delete query with user ID filter
	result, err := db.Exec(`
		DELETE FROM savings 
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return err
	}

	// Check if any rows were affected to validate successful deletion
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no savings goal found for user %s", userID)
	}

	return nil
}

