package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GitHubPayload estructura del payload de GitHub webhook
type GitHubPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"head_commit"`
}

const (
	WEBHOOK_SECRET_ENV = "GITHUB_WEBHOOK_SECRET"
	DEPLOY_SCRIPT_PATH = "/opt/hero_budget/webhook/deploy.sh"
	LOG_FILE_PATH      = "/opt/hero_budget/webhook/deployment.log"
	PORT               = ":9000"
)

// Función para verificar la firma del webhook
func verifySignature(payload []byte, signature string, secret string) bool {
	if secret == "" {
		log.Println("⚠️  WARNING: No webhook secret configured")
		return true // Permitir sin secret en desarrollo
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	expectedSignature := "sha256=" + expectedMAC

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// Función para loggear eventos
func logEvent(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s\n", timestamp, message)

	// Log a archivo
	file, err := os.OpenFile(LOG_FILE_PATH, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer file.Close()
		file.WriteString(logMessage)
	}

	// Log a consola
	log.Print(message)
}

// Función para ejecutar el script de deployment
func executeDeployment(payload GitHubPayload) error {
	logEvent(fmt.Sprintf("🚀 Iniciando deployment para commit %s por %s",
		payload.HeadCommit.ID[:8], payload.Pusher.Name))

	logEvent(fmt.Sprintf("📝 Commit message: %s", payload.HeadCommit.Message))

	// Verificar que el script existe
	if _, err := os.Stat(DEPLOY_SCRIPT_PATH); os.IsNotExist(err) {
		return fmt.Errorf("script de deployment no encontrado: %s", DEPLOY_SCRIPT_PATH)
	}

	// Ejecutar el script de deployment
	cmd := exec.Command("bash", DEPLOY_SCRIPT_PATH)
	cmd.Dir = "/opt/hero_budget"

	// Capturar output
	output, err := cmd.CombinedOutput()

	if err != nil {
		logEvent(fmt.Sprintf("❌ Error en deployment: %s", err.Error()))
		logEvent(fmt.Sprintf("📄 Output: %s", string(output)))
		return err
	}

	logEvent("✅ Deployment completado exitosamente")
	logEvent(fmt.Sprintf("📄 Output: %s", string(output)))

	return nil
}

// Handler principal del webhook
func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Solo aceptar POST requests
	if r.Method != "POST" {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	// Leer el cuerpo de la request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logEvent(fmt.Sprintf("❌ Error leyendo request body: %s", err.Error()))
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Verificar la firma del webhook
	signature := r.Header.Get("X-Hub-Signature-256")
	secret := os.Getenv(WEBHOOK_SECRET_ENV)

	if !verifySignature(body, signature, secret) {
		logEvent("❌ Firma del webhook inválida")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Verificar que es del repositorio correcto
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		logEvent(fmt.Sprintf("ℹ️  Evento ignorado: %s", event))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Event ignored"))
		return
	}

	// Parsear el payload
	var payload GitHubPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		logEvent(fmt.Sprintf("❌ Error parseando payload: %s", err.Error()))
		http.Error(w, "Error parsing payload", http.StatusBadRequest)
		return
	}

	// Verificar que es push a la rama main
	if payload.Ref != "refs/heads/main" {
		logEvent(fmt.Sprintf("ℹ️  Push ignorado - rama: %s", payload.Ref))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Branch ignored"))
		return
	}

	// Verificar que es el repositorio correcto
	expectedRepo := "jaivial/herobudget-backend"
	if payload.Repository.FullName != expectedRepo {
		logEvent(fmt.Sprintf("❌ Repositorio incorrecto: %s", payload.Repository.FullName))
		http.Error(w, "Wrong repository", http.StatusBadRequest)
		return
	}

	logEvent(fmt.Sprintf("📦 Push recibido en %s", payload.Repository.FullName))

	// Ejecutar deployment en goroutine para no bloquear la respuesta
	go func() {
		if err := executeDeployment(payload); err != nil {
			logEvent(fmt.Sprintf("❌ Deployment fallido: %s", err.Error()))
		}
	}()

	// Responder inmediatamente a GitHub
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deployment started"))
}

// Health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook server is running"))
}

// Handler para mostrar logs recientes
func logsHandler(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile(LOG_FILE_PATH)
	if err != nil {
		http.Error(w, "Error reading logs", http.StatusInternalServerError)
		return
	}

	lines := strings.Split(string(content), "\n")
	// Mostrar últimas 50 líneas
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(lines[start:], "\n")))
}

func main() {
	// Crear directorio de logs si no existe
	os.MkdirAll("/opt/hero_budget/webhook", 0755)

	logEvent("🔄 Iniciando servidor webhook para Hero Budget Backend")
	logEvent(fmt.Sprintf("🎯 Escuchando en puerto %s", PORT))

	// Configurar rutas
	http.HandleFunc("/webhook", webhookHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/logs", logsHandler)

	// Mensaje de inicio
	fmt.Printf("🚀 Servidor webhook iniciado en puerto %s\n", PORT)
	fmt.Printf("📋 Endpoints disponibles:\n")
	fmt.Printf("   • POST /webhook - Recibe webhooks de GitHub\n")
	fmt.Printf("   • GET  /health  - Health check\n")
	fmt.Printf("   • GET  /logs    - Ver logs recientes\n")
	fmt.Printf("📄 Logs en: %s\n", LOG_FILE_PATH)

	// Iniciar servidor
	if err := http.ListenAndServe(PORT, nil); err != nil {
		log.Fatalf("❌ Error iniciando servidor: %s", err.Error())
	}
}
