package common

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient estructura principal para manejar conexiones Redis
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// RedisConfig configuración para conexión Redis
type RedisConfig struct {
	Host     string        // Dirección del servidor Redis
	Port     string        // Puerto del servidor Redis
	Password string        // Contraseña de autenticación
	DB       int           // Base de datos Redis (0-15)
	PoolSize int           // Tamaño del pool de conexiones
	Timeout  time.Duration // Timeout para operaciones
}

var (
	// Cliente Redis global para reutilización
	globalRedisClient *RedisClient
	// Configuración por defecto
	defaultConfig = &RedisConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "Jva-Mvc-5171", // AUTH configurado en VPS
		DB:       0,
		PoolSize: 10,
		Timeout:  5 * time.Second,
	}
)

// NewRedisClient crea nueva instancia de cliente Redis
func NewRedisClient(config *RedisConfig) (*RedisClient, error) {
	// Usar configuración por defecto si no se proporciona
	if config == nil {
		config = defaultConfig
	}

	// Crear cliente Redis con pool de conexiones
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		ReadTimeout:  config.Timeout,
		WriteTimeout: config.Timeout,
		DialTimeout:  config.Timeout,
	})

	ctx := context.Background()

	// Verificar conectividad con Redis
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("error conectando a Redis: %v", err)
	}

	log.Printf("✓ Conexión Redis establecida: %s:%s", config.Host, config.Port)

	return &RedisClient{
		client: client,
		ctx:    ctx,
	}, nil
}

// GetGlobalClient obtiene instancia global del cliente Redis
func GetGlobalClient() (*RedisClient, error) {
	if globalRedisClient == nil {
		var err error
		globalRedisClient, err = NewRedisClient(defaultConfig)
		if err != nil {
			return nil, fmt.Errorf("error inicializando cliente Redis global: %v", err)
		}
	}
	return globalRedisClient, nil
}

// Set almacena valor en cache con TTL opcional
func (r *RedisClient) Set(key string, value interface{}, ttl time.Duration) error {
	err := r.client.Set(r.ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("error guardando en cache [%s]: %v", key, err)
	}
	log.Printf("🔄 Cache SET: %s (TTL: %v)", key, ttl)
	return nil
}

// Get obtiene valor del cache
func (r *RedisClient) Get(key string) (string, error) {
	val, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		log.Printf("🔍 Cache MISS: %s", key)
		return "", fmt.Errorf("key no encontrada: %s", key)
	} else if err != nil {
		return "", fmt.Errorf("error obteniendo del cache [%s]: %v", key, err)
	}
	log.Printf("✓ Cache HIT: %s", key)
	return val, nil
}

// Delete elimina key del cache
func (r *RedisClient) Delete(key string) error {
	err := r.client.Del(r.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("error eliminando del cache [%s]: %v", key, err)
	}
	log.Printf("🗑️ Cache DELETE: %s", key)
	return nil
}

// DeletePattern elimina múltiples keys que coincidan con patrón
func (r *RedisClient) DeletePattern(pattern string) error {
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("error buscando keys con patrón [%s]: %v", pattern, err)
	}

	if len(keys) > 0 {
		err = r.client.Del(r.ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("error eliminando keys con patrón [%s]: %v", pattern, err)
		}
		log.Printf("🗑️ Cache DELETE PATTERN: %s (%d keys)", pattern, len(keys))
	}

	return nil
}

// Exists verifica si existe una key en cache
func (r *RedisClient) Exists(key string) (bool, error) {
	exists, err := r.client.Exists(r.ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("error verificando existencia [%s]: %v", key, err)
	}
	return exists > 0, nil
}

// SetTTL establece tiempo de vida para una key existente
func (r *RedisClient) SetTTL(key string, ttl time.Duration) error {
	err := r.client.Expire(r.ctx, key, ttl).Err()
	if err != nil {
		return fmt.Errorf("error estableciendo TTL [%s]: %v", key, err)
	}
	log.Printf("⏰ Cache TTL SET: %s (%v)", key, ttl)
	return nil
}

// GetTTL obtiene tiempo de vida restante de una key
func (r *RedisClient) GetTTL(key string) (time.Duration, error) {
	ttl, err := r.client.TTL(r.ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("error obteniendo TTL [%s]: %v", key, err)
	}
	return ttl, nil
}

// Close cierra conexión Redis
func (r *RedisClient) Close() error {
	if r.client != nil {
		err := r.client.Close()
		if err != nil {
			return fmt.Errorf("error cerrando conexión Redis: %v", err)
		}
		log.Println("🔌 Conexión Redis cerrada")
	}
	return nil
}

// Ping verifica conectividad con Redis
func (r *RedisClient) Ping() error {
	_, err := r.client.Ping(r.ctx).Result()
	if err != nil {
		return fmt.Errorf("error de conectividad Redis: %v", err)
	}
	return nil
}

// Stats obtiene estadísticas del pool de conexiones
func (r *RedisClient) Stats() *redis.PoolStats {
	return r.client.PoolStats()
}

// Flush limpia toda la base de datos Redis (usar con precaución)
func (r *RedisClient) Flush() error {
	err := r.client.FlushDB(r.ctx).Err()
	if err != nil {
		return fmt.Errorf("error limpiando base de datos Redis: %v", err)
	}
	log.Println("🧹 Cache Redis limpiado completamente")
	return nil
}
