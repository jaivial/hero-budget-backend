// Script para corregir emojis corruptos en la base de datos
// Para ejecutar: cd backend/fix_scripts && go mod tidy && go run fix_emojis.go

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection with UTF-8 encoding support
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_case_sensitive_like=off&_encoding=UTF-8")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Set PRAGMA for UTF-8 encoding
	_, err = db.Exec(`PRAGMA encoding = "UTF-8";`)
	if err != nil {
		log.Printf("Failed to set UTF-8 encoding: %v", err)
	}

	// Get all categories with corrupted emojis
	rows, err := db.Query(`SELECT id, user_id, emoji FROM categories WHERE emoji = 'ð' OR emoji = 'ð ' OR emoji LIKE '%ð%'`)
	if err != nil {
		log.Fatalf("Failed to query categories: %v", err)
	}
	defer rows.Close()

	// Contador para categorías actualizadas
	var updatedCount int

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
		encoded := fmt.Sprintf("BASE64:%s", defaultEmoji)

		// Actualizar la categoría
		_, err := db.Exec(
			`UPDATE categories SET emoji = ? WHERE id = ? AND user_id = ?`,
			encoded, id, userId,
		)

		if err != nil {
			log.Printf("Error al actualizar categoría %d: %v", id, err)
			continue
		}

		log.Printf("Categoría %d actualizada con éxito: %s -> %s", id, emoji, defaultEmoji)
		updatedCount++
	}

	log.Printf("Proceso completado. %d categorías actualizadas.", updatedCount)
}
