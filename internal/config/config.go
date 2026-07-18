package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// дефолтный секрет, он же первый в блоклисте - в prod с ним не пустим
const defaultJWTSecret = "change-this-secret-in-production"

// меньше 32 байт в prod не даём, брутфорсится
const minJWTSecretLength = 32

// секреты-заглушки, которые нельзя тащить в прод (сравниваем без регистра)
var jwtSecretPlaceholders = []string{
	defaultJWTSecret,
	"your-secret-key-change-in-production",
	"change_me",
	"change-me",
	"change_me_to_strong_random_secret_in_production",
	"changeme",
	"secret",
	"password",
	"test",
}

func isProductionEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	return env == "production" || env == "prod"
}

// проверяем секрет только в prod, в dev пускаем что угодно
func validateJWTSecret(secret string, isProd bool) error {
	if !isProd {
		return nil
	}

	if secret == "" {
		return fmt.Errorf("JWT_SECRET must be set in production")
	}

	if len(secret) < minJWTSecretLength {
		return fmt.Errorf("JWT_SECRET must be at least %d bytes in production (got %d)",
			minJWTSecretLength, len(secret))
	}

	lower := strings.ToLower(secret)
	for _, placeholder := range jwtSecretPlaceholders {
		if lower == strings.ToLower(placeholder) {
			return fmt.Errorf("JWT_SECRET looks like a placeholder value; set a real random secret in production")
		}
	}

	return nil
}

// Config вся конфигурация приложения
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	Worker    WorkerConfig    `yaml:"worker"`
	Executor  ExecutorConfig  `yaml:"executor"`
	Storage   StorageConfig   `yaml:"storage"`
	JWT       JWTConfig       `yaml:"jwt"`
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	CORS      CORSConfig      `yaml:"cors"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type StorageConfig struct {
	ProgramsPath     string `yaml:"programs_path"`
	HostProgramsPath string `yaml:"host_programs_path"` // путь на хосте для docker-in-docker
	MaxFileSize      int64  `yaml:"max_file_size"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	BaseURL         string        `yaml:"base_url"` // для ссылок, напр. инвайты в команду
}

type DatabaseConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	User           string        `yaml:"user"`
	Password       string        `yaml:"password"`
	Name           string        `yaml:"name"`
	SSLMode        string        `yaml:"sslmode"`
	MaxConnections int           `yaml:"max_connections"`
	MaxIdle        int           `yaml:"max_idle"`
	MaxLifetime    time.Duration `yaml:"max_lifetime"`
	// сколько месяцев хранить партиции matches/rating_history.
	// 0 = не удаляем, чистка турнирных данных должна быть осознанной
	PartitionRetentionMonths int `yaml:"partition_retention_months"`
}

// DSN строка подключения к postgres в формате key=value
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// DSNURL то же самое но url-ом, нужно для golang-migrate
func (c DatabaseConfig) DSNURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

func (c RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type WorkerConfig struct {
	MinWorkers        int           `yaml:"min_workers"`
	MaxWorkers        int           `yaml:"max_workers"`
	QueueSize         int           `yaml:"queue_size"`
	Timeout           time.Duration `yaml:"timeout"`
	RetryAttempts     int           `yaml:"retry_attempts"`
	RetryDelay        time.Duration `yaml:"retry_delay"`
	AutoScaleInterval time.Duration `yaml:"auto_scale_interval"` // как часто проверяем пул, 0 = 2s
}

type ExecutorConfig struct {
	TJudgePath        string        `yaml:"tjudge_path"`
	DockerImage       string        `yaml:"docker_image"`
	Timeout           time.Duration `yaml:"timeout"`
	CPUQuota          int64         `yaml:"cpu_quota"`    // микросекунды на 100ms
	MemoryLimit       int64         `yaml:"memory_limit"` // в байтах
	PidsLimit         int64         `yaml:"pids_limit"`
	NetworkDisabled   bool          `yaml:"network_disabled"`
	DefaultIterations int           `yaml:"default_iterations"`
	Verbose           bool          `yaml:"verbose"`
	SeccompProfile    string        `yaml:"seccomp_profile"`
	AppArmorProfile   string        `yaml:"apparmor_profile"`
	CPUSetCPUs        string        `yaml:"cpuset_cpus"` // привязка к ядрам, напр "0-3"
	BuilderImage      string        `yaml:"builder_image"`
	CompileTimeout    time.Duration `yaml:"compile_timeout"`
}

type JWTConfig struct {
	Secret     string        `yaml:"secret"`
	AccessTTL  time.Duration `yaml:"access_ttl"`
	RefreshTTL time.Duration `yaml:"refresh_ttl"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
	Async  bool   `yaml:"async"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"max_age"`
}

type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	Burst             int  `yaml:"burst"`
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

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

	if c.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}
	if c.Redis.Port < 1 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Redis.Port)
	}

	if c.Worker.MinWorkers < 1 {
		return fmt.Errorf("worker min_workers must be positive")
	}
	if c.Worker.MaxWorkers < c.Worker.MinWorkers {
		return fmt.Errorf("worker max_workers must be >= min_workers")
	}
	if c.Worker.QueueSize < 1 {
		return fmt.Errorf("worker queue_size must be positive")
	}

	// jwt проверяем строго только в prod
	if err := validateJWTSecret(c.JWT.Secret, isProductionEnv()); err != nil {
		return err
	}
	if c.JWT.AccessTTL < 1*time.Minute {
		return fmt.Errorf("JWT access_ttl is too short")
	}

	validLevels := []string{"debug", "info", "warn", "error"}
	validLevel := slices.Contains(validLevels, c.Logging.Level)
	if !validLevel {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	return nil
}

// подбираем пул под воркеров, но не больше дефолтного лимита постгреса (100).
// если DB_MAX_CONNECTIONS задан явно - берётся он, это только дефолт
func recommendedDBPoolSize(workerMax int) int {
	const apiOverhead = 20
	const dbCeiling = 100
	val := min(int(float64(workerMax)*1.5)+apiOverhead, dbCeiling)
	if val < 10 {
		val = 10
	}
	return val
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загружаем .env файл если существует (игнорируем ошибку если файла нет)
	_ = godotenv.Load()

	// Если DB_MAX_CONNECTIONS не задан, подбираем по WORKER_MAX.
	workerMax := getEnvInt("WORKER_MAX", 1000)
	defaultPoolSize := recommendedDBPoolSize(workerMax)

	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvInt("API_PORT", 8080),
			ReadTimeout:     getEnvDuration("READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
			BaseURL:         getEnv("BASE_URL", "http://localhost:8080"),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           getEnvInt("DB_PORT", 5432),
			User:           getEnv("DB_USER", "tjudge"),
			Password:       getEnvOrFile("DB_PASSWORD", "secret"), // Поддержка Docker secrets
			Name:           getEnv("DB_NAME", "tjudge"),
			SSLMode:        getEnv("DB_SSLMODE", "disable"),
			MaxConnections: getEnvInt("DB_MAX_CONNECTIONS", defaultPoolSize),
			MaxIdle:        getEnvInt("DB_MAX_IDLE", defaultPoolSize/5), // 20% от max
			MaxLifetime:    getEnvDuration("DB_MAX_LIFETIME", 1*time.Hour),

			PartitionRetentionMonths: getEnvInt("DB_PARTITION_RETENTION_MONTHS", 0),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnvOrFile("REDIS_PASSWORD", ""), // Поддержка Docker secrets
			DB:       getEnvInt("REDIS_DB", 0),
			PoolSize: getEnvInt("REDIS_POOL_SIZE", 100),
		},
		Worker: WorkerConfig{
			MinWorkers:        getEnvInt("WORKER_MIN", 10),
			MaxWorkers:        getEnvInt("WORKER_MAX", 1000),
			QueueSize:         getEnvInt("WORKER_QUEUE_SIZE", 10000),
			Timeout:           getEnvDuration("WORKER_TIMEOUT", 90*time.Second),
			RetryAttempts:     getEnvInt("WORKER_RETRY_ATTEMPTS", 3),
			RetryDelay:        getEnvDuration("WORKER_RETRY_DELAY", 5*time.Second),
			AutoScaleInterval: getEnvDuration("WORKER_AUTOSCALE_INTERVAL", 2*time.Second),
		},
		Executor: ExecutorConfig{
			TJudgePath:        getEnv("TJUDGE_PATH", "tjudge-cli"),
			DockerImage:       getEnv("EXECUTOR_DOCKER_IMAGE", "tjudge-cli:latest"),
			Timeout:           getEnvDuration("EXECUTOR_TIMEOUT", 60*time.Second),
			CPUQuota:          int64(getEnvInt("EXECUTOR_CPU_QUOTA", 100000)),
			MemoryLimit:       int64(getEnvInt("EXECUTOR_MEMORY_LIMIT", 536870912)),
			PidsLimit:         int64(getEnvInt("EXECUTOR_PIDS_LIMIT", 100)),
			NetworkDisabled:   getEnvBool("EXECUTOR_NETWORK_DISABLED", true),
			DefaultIterations: getEnvInt("EXECUTOR_DEFAULT_ITERATIONS", 100),
			Verbose:           getEnvBool("EXECUTOR_VERBOSE", false),
			SeccompProfile:    getEnv("EXECUTOR_SECCOMP_PROFILE", ""),
			AppArmorProfile:   getEnv("EXECUTOR_APPARMOR_PROFILE", ""),
			BuilderImage:      getEnv("EXECUTOR_BUILDER_IMAGE", "tjudge-builder:latest"),
			CompileTimeout:    getEnvDuration("EXECUTOR_COMPILE_TIMEOUT", 120*time.Second),
			CPUSetCPUs:        getEnv("EXECUTOR_CPUSET_CPUS", ""),
		},
		Storage: StorageConfig{
			ProgramsPath:     getEnv("PROGRAMS_PATH", "/data/programs"),
			HostProgramsPath: getEnv("HOST_PROGRAMS_PATH", ""),            // Если пусто, используется ProgramsPath
			MaxFileSize:      int64(getEnvInt("MAX_FILE_SIZE", 10485760)), // 10MB
		},
		JWT: JWTConfig{
			Secret:     getEnvOrFile("JWT_SECRET", defaultJWTSecret),      // Поддержка Docker secrets
			AccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 24*time.Hour),    // 24 часа активной сессии
			RefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour), // 7 дней неактивности
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
			AllowedOrigins: splitAndTrim(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         getEnvInt("CORS_MAX_AGE", 3600),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", false), // Disabled by default for development
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

// режем csv по запятой, пустые куски выкидываем
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

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

// сначала пробуем обычную переменную, потом KEY_FILE (docker secrets)
func getEnvOrFile(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	fileKey := key + "_FILE"
	if filePath := os.Getenv(fileKey); filePath != "" {
		content, err := os.ReadFile(filePath) // #nosec G304 -- путь из env, это docker secrets
		if err == nil {
			return strings.TrimSpace(string(content)) // убираем хвостовой перевод строки
		}
	}

	return defaultValue
}
