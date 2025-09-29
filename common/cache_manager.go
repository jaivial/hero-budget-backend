package common

import (
	"encoding/json"
	"fmt"
	"strings" // Agregado para soporte de operaciones con strings
	"time"
)

// CacheManager gestiona operaciones de cache granular con invalidación específica
type CacheManager struct {
	redis *RedisClient
}

// CacheConfig configuración de TTL por tipo de datos
type CacheConfig struct {
	UserDataTTL    time.Duration // TTL para datos de usuario
	IncomeDataTTL  time.Duration // TTL para datos de ingresos
	ExpenseDataTTL time.Duration // TTL para datos de gastos
	BillsDataTTL   time.Duration // TTL para datos de facturas
	DashboardTTL   time.Duration // TTL para datos del dashboard
	SavingsDataTTL time.Duration // TTL para datos de ahorros
}

// TTL por defecto para diferentes tipos de datos
var defaultCacheConfig = &CacheConfig{
	UserDataTTL:    30 * time.Minute, // Datos de usuario: 30 min
	IncomeDataTTL:  10 * time.Minute, // Ingresos: 10 min (frecuente)
	ExpenseDataTTL: 10 * time.Minute, // Gastos: 10 min (frecuente)
	BillsDataTTL:   15 * time.Minute, // Facturas: 15 min (medio)
	DashboardTTL:   5 * time.Minute,  // Dashboard: 5 min (muy dinámico)
	SavingsDataTTL: 20 * time.Minute, // Ahorros: 20 min (menos frecuente)
}

// NewCacheManager crea nueva instancia del gestor de cache
func NewCacheManager() (*CacheManager, error) {
	redisClient, err := GetGlobalClient()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo cliente Redis: %v", err)
	}

	return &CacheManager{
		redis: redisClient,
	}, nil
}

// CacheUserData almacena datos de usuario en cache
func (cm *CacheManager) CacheUserData(userID string, data interface{}) error {
	key := KB.BuildUserKey(userID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos de usuario: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.UserDataTTL)
}

// GetUserData obtiene datos de usuario del cache
func (cm *CacheManager) GetUserData(userID string, result interface{}) error {
	key := KB.BuildUserKey(userID)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// CacheIncomeData almacena datos de ingresos en cache
func (cm *CacheManager) CacheIncomeData(userID, period string, data interface{}) error {
	key := KB.BuildIncomeKey(userID, period)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos de ingresos: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.IncomeDataTTL)
}

// GetIncomeData obtiene datos de ingresos del cache
func (cm *CacheManager) GetIncomeData(userID, period string, result interface{}) error {
	key := KB.BuildIncomeKey(userID, period)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// CacheDashboardData almacena datos del dashboard en cache
func (cm *CacheManager) CacheDashboardData(userID, period string, data interface{}) error {
	key := KB.BuildDashboardKey(userID, period)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos del dashboard: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.DashboardTTL)
}

// GetDashboardData obtiene datos del dashboard del cache
func (cm *CacheManager) GetDashboardData(userID, period string, result interface{}) error {
	key := KB.BuildDashboardKey(userID, period)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// CacheExpenseData almacena datos de gastos en cache
// Serializa los datos y los almacena con TTL específico para gastos
func (cm *CacheManager) CacheExpenseData(userID, period string, data interface{}) error {
	key := KB.BuildExpenseKey(userID, period)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos de gastos: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.ExpenseDataTTL)
}

// GetExpenseData obtiene datos de gastos del cache
// Deserializa los datos almacenados en el resultado proporcionado
func (cm *CacheManager) GetExpenseData(userID, period string, result interface{}) error {
	key := KB.BuildExpenseKey(userID, period)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// CacheBillsData almacena datos de facturas en cache
// Serializa los datos y los almacena con TTL específico para facturas
func (cm *CacheManager) CacheBillsData(userID, period string, data interface{}) error {
	key := KB.BuildBillsKey(userID, period)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos de facturas: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.BillsDataTTL)
}

// GetBillsData obtiene datos de facturas del cache
// Deserializa los datos almacenados en el resultado proporcionado
func (cm *CacheManager) GetBillsData(userID, period string, result interface{}) error {
	key := KB.BuildBillsKey(userID, period)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// CacheSavingsData almacena datos de ahorros en cache
// Serializa los datos y los almacena con TTL específico para ahorros
func (cm *CacheManager) CacheSavingsData(userID string, data interface{}) error {
	key := KB.BuildSavingsKey(userID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos de ahorros: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.SavingsDataTTL)
}

// GetSavingsData obtiene datos de ahorros del cache
// Deserializa los datos almacenados en el resultado proporcionado
func (cm *CacheManager) GetSavingsData(userID string, result interface{}) error {
	key := KB.BuildSavingsKey(userID)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// CacheCashBankData almacena datos de distribución cash/bank en cache
// Serializa los datos y los almacena con TTL de datos de usuario
func (cm *CacheManager) CacheCashBankData(userID string, data interface{}) error {
	key := KB.BuildCashBankKey(userID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializando datos de cash/bank: %v", err)
	}

	return cm.redis.Set(key, jsonData, defaultCacheConfig.UserDataTTL)
}

// GetCashBankData obtiene datos de distribución cash/bank del cache
// Deserializa los datos almacenados en el resultado proporcionado
func (cm *CacheManager) GetCashBankData(userID string, result interface{}) error {
	key := KB.BuildCashBankKey(userID)
	data, err := cm.redis.Get(key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), result)
}

// GetCacheStats obtiene estadísticas del cache Redis
// Proporciona métricas de rendimiento del cache para monitoreo
func (cm *CacheManager) GetCacheStats() (*CacheStats, error) {
	stats := cm.redis.Stats()

	return &CacheStats{
		Hits:       stats.Hits,
		Misses:     stats.Misses,
		Timeouts:   stats.Timeouts,
		TotalConns: stats.TotalConns,
		IdleConns:  stats.IdleConns,
		StaleConns: stats.StaleConns,
	}, nil
}

// CacheStats estadísticas del cache Redis para monitoreo de rendimiento
type CacheStats struct {
	Hits       uint32 `json:"hits"`        // Número de hits del cache
	Misses     uint32 `json:"misses"`      // Número de misses del cache
	Timeouts   uint32 `json:"timeouts"`    // Número de timeouts
	TotalConns uint32 `json:"total_conns"` // Conexiones totales
	IdleConns  uint32 `json:"idle_conns"`  // Conexiones inactivas
	StaleConns uint32 `json:"stale_conns"` // Conexiones obsoletas
}

// IsValidKey verifica si una key tiene el formato correcto
// Utiliza el separador ':' para validar que la key tenga al menos dos partes no vacías
func (cm *CacheManager) IsValidKey(key string) bool {
	parts := strings.Split(key, ":")
	return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
}
