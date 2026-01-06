package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	Worker    WorkerConfig    `yaml:"worker"`
	Executor  ExecutorConfig  `yaml:"executor"`
	JWT       JWTConfig       `yaml:"jwt"`
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	CORS      CORSConfig      `yaml:"cors"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// ServerConfig - конфигурация HTTP сервера
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig - конфигурация PostgreSQL
type DatabaseConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	User           string        `yaml:"user"`
	Password       string        `yaml:"password"`
	Name           string        `yaml:"name"`
	MaxConnections int           `yaml:"max_connections"`
	MaxIdle        int           `yaml:"max_idle"`
	MaxLifetime    time.Duration `yaml:"max_lifetime"`
}

// DSN возвращает строку подключения к PostgreSQL (формат key=value)
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password, c.Name,
	)
}

// DSNURL возвращает строку подключения в URL формате (для golang-migrate)
func (c DatabaseConfig) DSNURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Name,
	)
}

// RedisConfig - конфигурация Redis
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// Address возвращает адрес Redis
func (c RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// WorkerConfig - конфигурация worker pool
type WorkerConfig struct {
	MinWorkers    int           `yaml:"min_workers"`
	MaxWorkers    int           `yaml:"max_workers"`
	QueueSize     int           `yaml:"queue_size"`
	Timeout       time.Duration `yaml:"timeout"`
	RetryAttempts int           `yaml:"retry_attempts"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
}

// ExecutorConfig - конфигурация исполнителя матчей
type ExecutorConfig struct {
	TJudgePath        string        `yaml:"tjudge_path"`        // Путь к tjudge-cli внутри контейнера
	DockerImage       string        `yaml:"docker_image"`       // Имя Docker образа для tjudge-cli
	Timeout           time.Duration `yaml:"timeout"`            // Таймаут выполнения матча
	CPUQuota          int64         `yaml:"cpu_quota"`          // Лимит CPU (микросекунды на 100ms)
	MemoryLimit       int64         `yaml:"memory_limit"`       // Лимит памяти в байтах
	PidsLimit         int64         `yaml:"pids_limit"`         // Лимит процессов
	NetworkDisabled   bool          `yaml:"network_disabled"`   // Отключить сеть
	DefaultIterations int           `yaml:"default_iterations"` // Количество итераций по умолчанию
	Verbose           bool          `yaml:"verbose"`            // Включить verbose вывод
	SeccompProfile    string        `yaml:"seccomp_profile"`    // Путь к seccomp профилю
	AppArmorProfile   string        `yaml:"apparmor_profile"`   // Имя AppArmor профиля
	CPUSetCPUs        string        `yaml:"cpuset_cpus"`        // Привязка к ядрам CPU (например "0-3")
}

// JWTConfig - конфигурация JWT токенов
type JWTConfig struct {
	Secret     string        `yaml:"secret"`
	AccessTTL  time.Duration `yaml:"access_ttl"`
	RefreshTTL time.Duration `yaml:"refresh_ttl"`
}

// LoggingConfig - конфигурация логирования
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
	Async  bool   `yaml:"async"` // Асинхронное логирование с буферизацией
}

// MetricsConfig - конфигурация метрик
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// CORSConfig - конфигурация CORS
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"max_age"`
}

// RateLimitConfig - конфигурация rate limiting
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	Burst             int  `yaml:"burst"`
}

// Validate валидирует конфигурацию
func (c *Config) Validate() error {
	// Валидация Server
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Валидация Database
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}
	if c.Database.MaxConnections < 1 {
		return fmt.Errorf("database max_connections must be positive")
	}

	// Валидация Redis
	if c.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}
	if c.Redis.Port < 1 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Redis.Port)
	}

	// Валидация Worker
	if c.Worker.MinWorkers < 1 {
		return fmt.Errorf("worker min_workers must be positive")
	}
	if c.Worker.MaxWorkers < c.Worker.MinWorkers {
		return fmt.Errorf("worker max_workers must be >= min_workers")
	}
	if c.Worker.QueueSize < 1 {
		return fmt.Errorf("worker queue_size must be positive")
	}

	// Валидация JWT
	if c.JWT.Secret == "" || c.JWT.Secret == "change-this-secret-in-production" {
		// В production это должно быть ошибкой
		env := os.Getenv("ENVIRONMENT")
		if env == "production" || env == "prod" {
			return fmt.Errorf("JWT secret must be changed in production")
		}
	}
	if c.JWT.AccessTTL < 1*time.Minute {
		return fmt.Errorf("JWT access_ttl is too short")
	}

	// Валидация Logging
	validLevels := []string{"debug", "info", "warn", "error"}
	validLevel := false
	for _, level := range validLevels {
		if c.Logging.Level == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	return nil
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загружаем .env файл если существует
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvInt("API_PORT", 8080),
			ReadTimeout:     getEnvDuration("READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           getEnvInt("DB_PORT", 5432),
			User:           getEnv("DB_USER", "tjudge"),
			Password:       getEnvOrFile("DB_PASSWORD", "secret"), // Поддержка Docker secrets
			Name:           getEnv("DB_NAME", "tjudge"),
			MaxConnections: getEnvInt("DB_MAX_CONNECTIONS", 50),
			MaxIdle:        getEnvInt("DB_MAX_IDLE", 10),
			MaxLifetime:    getEnvDuration("DB_MAX_LIFETIME", 1*time.Hour),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnvOrFile("REDIS_PASSWORD", ""), // Поддержка Docker secrets
			DB:       getEnvInt("REDIS_DB", 0),
			PoolSize: getEnvInt("REDIS_POOL_SIZE", 100),
		},
		Worker: WorkerConfig{
			MinWorkers:    getEnvInt("WORKER_MIN", 10),
			MaxWorkers:    getEnvInt("WORKER_MAX", 1000),
			QueueSize:     getEnvInt("WORKER_QUEUE_SIZE", 10000),
			Timeout:       getEnvDuration("WORKER_TIMEOUT", 30*time.Second),
			RetryAttempts: getEnvInt("WORKER_RETRY_ATTEMPTS", 3),
			RetryDelay:    getEnvDuration("WORKER_RETRY_DELAY", 5*time.Second),
		},
		Executor: ExecutorConfig{
			TJudgePath:        getEnv("TJUDGE_PATH", "tjudge-cli"),
			DockerImage:       getEnv("EXECUTOR_DOCKER_IMAGE", "tjudge-cli:latest"),
			Timeout:           getEnvDuration("EXECUTOR_TIMEOUT", 60*time.Second),
			CPUQuota:          int64(getEnvInt("EXECUTOR_CPU_QUOTA", 100000)),
			MemoryLimit:       int64(getEnvInt("EXECUTOR_MEMORY_LIMIT", 536870912)),
			PidsLimit:         int64(getEnvInt("EXECUTOR_PIDS_LIMIT", 100)),
			NetworkDisabled:   getEnvBool("EXECUTOR_NETWORK_DISABLED", true),
			DefaultIterations: getEnvInt("EXECUTOR_DEFAULT_ITERATIONS", 1000),
			Verbose:           getEnvBool("EXECUTOR_VERBOSE", false),
			SeccompProfile:    getEnv("EXECUTOR_SECCOMP_PROFILE", ""),
			AppArmorProfile:   getEnv("EXECUTOR_APPARMOR_PROFILE", ""),
			CPUSetCPUs:        getEnv("EXECUTOR_CPUSET_CPUS", ""),
		},
		JWT: JWTConfig{
			Secret:     getEnvOrFile("JWT_SECRET", "change-this-secret-in-production"), // Поддержка Docker secrets
			AccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 168*time.Hour),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
			Output: getEnv("LOG_OUTPUT", "stdout"),
			Async:  getEnvBool("LOG_ASYNC", true), // По умолчанию async для production
		},
		Metrics: MetricsConfig{
			Enabled: getEnvBool("METRICS_ENABLED", true),
			Port:    getEnvInt("METRICS_PORT", 9090),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         getEnvInt("CORS_MAX_AGE", 3600),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvInt("RATE_LIMIT_RPM", 100),
			Burst:             getEnvInt("RATE_LIMIT_BURST", 200),
		},
	}

	// Валидируем конфигурацию
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Вспомогательные функции для чтения переменных окружения

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getEnvOrFile читает значение из переменной окружения или из файла
// Сначала проверяет KEY, затем KEY_FILE
// Это поддерживает Docker secrets
func getEnvOrFile(key, defaultValue string) string {
	// Сначала проверяем обычную переменную
	if value := os.Getenv(key); value != "" {
		return value
	}

	// Затем проверяем переменную с суффиксом _FILE
	fileKey := key + "_FILE"
	if filePath := os.Getenv(fileKey); filePath != "" {
		content, err := os.ReadFile(filePath)
		if err == nil {
			// Убираем trailing newline
			return strings.TrimSpace(string(content))
		}
	}

	return defaultValue
}
