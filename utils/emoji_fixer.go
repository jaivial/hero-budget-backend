package utils

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
)

// FixEmojis ejecuta el proceso de reparación de emojis en la base de datos
func FixEmojis() error {
	// Obtener la ruta de trabajo actual
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error al obtener el directorio actual: %v", err)
	}

	// Construir ruta absoluta a la base de datos
	dbPath := filepath.Join(cwd, "google_auth", "users.db")
	log.Printf("Usando base de datos en: %s", dbPath)

	// Abrir la conexión a la base de datos con soporte UTF-8
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_case_sensitive_like=off&_encoding=UTF-8")
	if err != nil {
		return fmt.Errorf("error al abrir la base de datos: %v", err)
	}
	defer db.Close()

	// Verificar la conexión
	if err = db.Ping(); err != nil {
		return fmt.Errorf("error al comprobar la conexión con la base de datos: %v", err)
	}

	// Configurar PRAGMA para UTF-8
	_, err = db.Exec(`PRAGMA encoding = "UTF-8";`)
	if err != nil {
		log.Printf("Error al configurar la codificación UTF-8: %v", err)
	}

	// Buscar todas las categorías con emojis corruptos
	rows, err := db.Query(`SELECT id, user_id, emoji FROM categories WHERE emoji NOT LIKE 'BASE64:%' AND emoji != '📊'`)
	if err != nil {
		return fmt.Errorf("error al consultar categorías: %v", err)
	}
	defer rows.Close()

	// Contador para categorías actualizadas
	var updatedCount int

	// Emojis predeterminados para asignar variedad
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

		// Verificar si es un emoji corrupto conocido
		isCorrupted := (emoji == "ð" || emoji == "ð " ||
			strings.Contains(emoji, "ð") ||
			strings.Contains(emoji, "â") ||
			!utf8.ValidString(emoji))

		if isCorrupted {
			log.Printf("Emoji corrupto detectado en categoría %d: '%s'", id, emoji)

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

			log.Printf("Categoría %d actualizada con éxito: %s -> %s", id, emoji, encodedEmoji)
			updatedCount++
		} else {
			// Si no es un emoji corrupto pero tampoco está codificado, codificarlo
			if !strings.HasPrefix(emoji, "BASE64:") && emoji != "📊" {
				encodedEmoji := encodeEmoji(emoji)

				// Actualizar la categoría
				_, err := db.Exec(
					`UPDATE categories SET emoji = ? WHERE id = ? AND user_id = ?`,
					encodedEmoji, id, userId,
				)

				if err != nil {
					log.Printf("Error al codificar emoji de categoría %d: %v", id, err)
					continue
				}

				log.Printf("Categoría %d codificada con éxito: %s -> %s", id, emoji, encodedEmoji)
				updatedCount++
			}
		}
	}

	log.Printf("Proceso completado. %d categorías actualizadas.", updatedCount)

	// Validar que todas las categorías ahora tengan emojis correctos
	validateRows, err := db.Query(`SELECT id, emoji FROM categories`)
	if err != nil {
		return fmt.Errorf("error al validar categorías: %v", err)
	}
	defer validateRows.Close()

	var validCount, invalidCount int
	for validateRows.Next() {
		var id int
		var emoji string

		if err := validateRows.Scan(&id, &emoji); err != nil {
			log.Printf("Error al escanear fila de validación: %v", err)
			continue
		}

		// Verificar si el emoji es válido
		if strings.HasPrefix(emoji, "BASE64:") || emoji == "📊" || utf8.ValidString(emoji) {
			validCount++
		} else {
			invalidCount++
			log.Printf("Categoría %d aún tiene emoji inválido: %s", id, emoji)
		}
	}

	log.Printf("Validación completada. %d categorías válidas, %d inválidas.", validCount, invalidCount)
	return nil
}

// encodeEmoji codifica un emoji como Base64 para almacenamiento seguro
func encodeEmoji(emoji string) string {
	if emoji == "" {
		return "📊" // Emoji predeterminado
	}

	// Solo codificar si parece un emoji (en general, emojis tienen bytes especiales)
	needsEncoding := false
	for _, r := range emoji {
		if r > 127 { // Caracteres fuera del rango ASCII estándar
			needsEncoding = true
			break
		}
	}

	if needsEncoding {
		encoded := base64.StdEncoding.EncodeToString([]byte(emoji))
		result := "BASE64:" + encoded
		log.Printf("DEBUG - Emoji codificado: '%s' -> '%s'", emoji, result)
		return result
	}

	return emoji
}
