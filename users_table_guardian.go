package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// UsersTableGuardian - Protector de la tabla users contra eliminaciones accidentales
type UsersTableGuardian struct {
	db             *sql.DB
	dbPath         string
	backupDir      string
	monitoringFile string
}

// NewUsersTableGuardian crea una nueva instancia del guardián
func NewUsersTableGuardian() (*UsersTableGuardian, error) {
	// Obtener directorio actual
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

	// Configurar rutas
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	backupDir := filepath.Join(cwd, "..", "google_auth", "backups")
	monitoringFile := filepath.Join(cwd, "users_monitoring.log")

	// Crear directorio de backups si no existe
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Abrir conexión a la base de datos
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &UsersTableGuardian{
		db:             db,
		dbPath:         dbPath,
		backupDir:      backupDir,
		monitoringFile: monitoringFile,
	}, nil
}

// CreateBackup crea un backup de la base de datos
func (g *UsersTableGuardian) CreateBackup() error {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(g.backupDir, fmt.Sprintf("users_backup_%s.db", timestamp))

	// Crear comando de backup usando SQLite
	backupDB, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup database: %v", err)
	}
	defer backupDB.Close()

	// Copiar datos de la tabla users
	rows, err := g.db.Query("SELECT * FROM users")
	if err != nil {
		return fmt.Errorf("failed to query users table: %v", err)
	}
	defer rows.Close()

	// Crear tabla users en el backup
	_, err = backupDB.Exec(`
		CREATE TABLE users (
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
		return fmt.Errorf("failed to create users table in backup: %v", err)
	}

	// Copiar todos los datos
	for rows.Next() {
		var (
			id                 int
			googleID           sql.NullString
			appleID            sql.NullString
			email              string
			password           string
			name               string
			givenName          string
			familyName         string
			picture            string
			profileImageBlob   sql.NullString
			locale             string
			verifiedEmail      bool
			verificationCode   string
			userType           string
			createdAt          string
			updatedAt          string
			resetToken         sql.NullString
			resetExpires       sql.NullString
		)

		err := rows.Scan(&id, &googleID, &appleID, &email, &password, &name, &givenName, &familyName,
			&picture, &profileImageBlob, &locale, &verifiedEmail, &verificationCode, &userType,
			&createdAt, &updatedAt, &resetToken, &resetExpires)
		if err != nil {
			return fmt.Errorf("failed to scan user row: %v", err)
		}

		// Insertar en backup
		_, err = backupDB.Exec(`
			INSERT INTO users (id, google_id, apple_id, email, password, name, given_name, family_name,
				picture, profile_image_blob, locale, verified_email, verification_code, type,
				created_at, updated_at, reset_token, reset_expires)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, googleID, appleID, email, password, name, givenName, familyName,
			picture, profileImageBlob, locale, verifiedEmail, verificationCode, userType,
			createdAt, updatedAt, resetToken, resetExpires)
		if err != nil {
			return fmt.Errorf("failed to insert user into backup: %v", err)
		}
	}

	log.Printf("✅ Backup creado: %s", backupPath)
	return nil
}

// GetUserCount obtiene el número actual de usuarios
func (g *UsersTableGuardian) GetUserCount() (int, error) {
	var count int
	err := g.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// LogChange registra cambios en la tabla users
func (g *UsersTableGuardian) LogChange(previousCount, currentCount int, action string) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s: %d -> %d usuarios\n", timestamp, action, previousCount, currentCount)

	file, err := os.OpenFile(g.monitoringFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(logEntry)
	return err
}

// MonitorChanges monitorea cambios en la tabla users
func (g *UsersTableGuardian) MonitorChanges() {
	log.Println("🔍 Iniciando monitoreo de tabla users...")

	previousCount, err := g.GetUserCount()
	if err != nil {
		log.Printf("Error obteniendo conteo inicial: %v", err)
		return
	}

	log.Printf("📊 Conteo inicial de usuarios: %d", previousCount)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		currentCount, err := g.GetUserCount()
		if err != nil {
			log.Printf("Error obteniendo conteo actual: %v", err)
			continue
		}

		if currentCount != previousCount {
			// ¡Cambio detectado!
			if currentCount < previousCount {
				log.Printf("🚨 ALERTA: Usuarios eliminados! %d -> %d", previousCount, currentCount)
				g.LogChange(previousCount, currentCount, "USUARIOS_ELIMINADOS")

				// Crear backup de emergencia
				if err := g.CreateBackup(); err != nil {
					log.Printf("Error creando backup de emergencia: %v", err)
				}
			} else {
				log.Printf("✅ Nuevos usuarios añadidos: %d -> %d", previousCount, currentCount)
				g.LogChange(previousCount, currentCount, "USUARIOS_AÑADIDOS")
			}

			previousCount = currentCount
		}
	}
}

// RestoreFromBackup restaura desde el backup más reciente
func (g *UsersTableGuardian) RestoreFromBackup() error {
	// Buscar el backup más reciente
	files, err := filepath.Glob(filepath.Join(g.backupDir, "users_backup_*.db"))
	if err != nil {
		return fmt.Errorf("failed to find backup files: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no backup files found")
	}

	// Obtener el archivo más reciente
	var latestFile string
	var latestTime time.Time
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file
		}
	}

	log.Printf("🔄 Restaurando desde backup: %s", latestFile)

	// Abrir backup
	backupDB, err := sql.Open("sqlite3", latestFile)
	if err != nil {
		return fmt.Errorf("failed to open backup database: %v", err)
	}
	defer backupDB.Close()

	// Verificar que el backup tiene datos
	var backupCount int
	err = backupDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&backupCount)
	if err != nil {
		return fmt.Errorf("failed to count users in backup: %v", err)
	}

	if backupCount == 0 {
		return fmt.Errorf("backup file is empty")
	}

	log.Printf("📊 Backup contiene %d usuarios", backupCount)

	// Eliminar datos actuales de la tabla users
	_, err = g.db.Exec("DELETE FROM users")
	if err != nil {
		return fmt.Errorf("failed to clear users table: %v", err)
	}

	// Copiar datos desde backup
	rows, err := backupDB.Query("SELECT * FROM users")
	if err != nil {
		return fmt.Errorf("failed to query backup users: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                 int
			googleID           sql.NullString
			appleID            sql.NullString
			email              string
			password           string
			name               string
			givenName          string
			familyName         string
			picture            string
			profileImageBlob   sql.NullString
			locale             string
			verifiedEmail      bool
			verificationCode   string
			userType           string
			createdAt          string
			updatedAt          string
			resetToken         sql.NullString
			resetExpires       sql.NullString
		)

		err := rows.Scan(&id, &googleID, &appleID, &email, &password, &name, &givenName, &familyName,
			&picture, &profileImageBlob, &locale, &verifiedEmail, &verificationCode, &userType,
			&createdAt, &updatedAt, &resetToken, &resetExpires)
		if err != nil {
			return fmt.Errorf("failed to scan backup user row: %v", err)
		}

		// Insertar en tabla principal
		_, err = g.db.Exec(`
			INSERT INTO users (id, google_id, apple_id, email, password, name, given_name, family_name,
				picture, profile_image_blob, locale, verified_email, verification_code, type,
				created_at, updated_at, reset_token, reset_expires)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, googleID, appleID, email, password, name, givenName, familyName,
			picture, profileImageBlob, locale, verifiedEmail, verificationCode, userType,
			createdAt, updatedAt, resetToken, resetExpires)
		if err != nil {
			return fmt.Errorf("failed to insert user from backup: %v", err)
		}
	}

	log.Printf("✅ Restauración completada: %d usuarios restaurados", backupCount)
	return nil
}

func main() {
	guardian, err := NewUsersTableGuardian()
	if err != nil {
		log.Fatalf("Failed to create guardian: %v", err)
	}
	defer guardian.db.Close()

	// Crear backup inicial
	if err := guardian.CreateBackup(); err != nil {
		log.Printf("Error creando backup inicial: %v", err)
	}

	// Verificar argumentos de línea de comandos
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--restore":
			if err := guardian.RestoreFromBackup(); err != nil {
				log.Fatalf("Error restaurando backup: %v", err)
			}
			return
		case "--backup":
			if err := guardian.CreateBackup(); err != nil {
				log.Fatalf("Error creando backup: %v", err)
			}
			return
		case "--monitor":
			guardian.MonitorChanges()
			return
		}
	}

	// Por defecto, mostrar información
	count, err := guardian.GetUserCount()
	if err != nil {
		log.Fatalf("Error obteniendo conteo de usuarios: %v", err)
	}

	fmt.Printf("🔒 Users Table Guardian\n")
	fmt.Printf("📊 Usuarios actuales: %d\n", count)
	fmt.Printf("📁 Directorio de backups: %s\n", guardian.backupDir)
	fmt.Printf("📋 Archivo de monitoreo: %s\n", guardian.monitoringFile)
	fmt.Printf("\nOpciones:\n")
	fmt.Printf("  --monitor   Monitorear cambios en tiempo real\n")
	fmt.Printf("  --backup    Crear backup manual\n")
	fmt.Printf("  --restore   Restaurar desde backup más reciente\n")
}