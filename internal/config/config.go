package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// дефолтный секрет, он же первый в блоклисте - в prod с ним не пустит
const defaultJWTSecret = "change-this-secret-in-production"

// меньше 32 байт в prod нельзя, брутфорсится
const minJWTSecretLength = 32

// на сколько WORKER_TIMEOUT должен превышать EXECUTOR_TIMEOUT
const workerTimeoutMargin = 20 * time.Second

// минимальный лимит памяти контейнера, который принимает docker
const minExecutorMemory = 6 * 1024 * 1024

// секреты-заглушки, которые нельзя тащить в прод (сравнение без регистра)
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

// секрет проверяется только в prod, в dev проходит что угодно
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
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Worker    WorkerConfig
	Executor  ExecutorConfig
	Storage   StorageConfig
	JWT       JWTConfig
	Logging   LoggingConfig
	Metrics   MetricsConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

type StorageConfig struct {
	ProgramsPath     string
	HostProgramsPath string // путь на хосте для docker-in-docker
}

type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	BaseURL         string // для ссылок, напр. инвайты в комманду
}

type DatabaseConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	Name           string
	SSLMode        string
	MaxConnections int
	MaxIdle        int
	MaxLifetime    time.Duration
	// сколько месяцев хранить партиции matches/rating_history.
	// 0 = без удаления, чистка турнирных данных должна быть осознанной
	PartitionRetentionMonths int
}

// DSN строка подключения к postgres в формате key=value. значения в кавычках с
// экранированием, иначе пароль с пробелом или кавычкой ломает разбор.
// timezone=UTC: колонки TIMESTAMP без зоны заполняются и NOW() сервера, и временем из
// Go; при другой зоне сессии время съезжает на смещение, а с ним границы партиций
func (c DatabaseConfig) DSN() string {
	quote := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	q := func(v string) string { return "'" + quote.Replace(v) + "'" }
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s timezone=UTC",
		q(c.Host), c.Port, q(c.User), q(c.Password), q(c.Name), q(c.SSLMode),
	)
}

// DSNURL то же самое но url-ом, нужно для golang-migrate
func (c DatabaseConfig) DSNURL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     "/" + c.Name,
		RawQuery: url.Values{"sslmode": {c.SSLMode}, "timezone": {"UTC"}}.Encode(),
	}
	return u.String()
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

func (c RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type WorkerConfig struct {
	MinWorkers        int
	MaxWorkers        int
	Timeout           time.Duration
	RetryAttempts     int
	RetryDelay        time.Duration
	AutoScaleInterval time.Duration // как часто проверяется пул, 0 = 2s
}

// StuckThreshold - после скольких секунд в running матч считается брошенным.
// живой воркер держит матч не дольше Timeout, запас покрывает запись результата
func (w WorkerConfig) StuckThreshold() time.Duration {
	return w.Timeout + 30*time.Second
}

type ExecutorConfig struct {
	DockerImage       string
	Timeout           time.Duration
	CPUQuota          int64 // микросекунды на 100ms
	MemoryLimit       int64 // в байтах
	PidsLimit         int64
	DefaultIterations int
	Verbose           bool
	SeccompProfile    string
	AppArmorProfile   string
	CPUSetCPUs        string // привязка к ядрам, напр "0-3"
	BuilderImage      string
	CompileTimeout    time.Duration
	CompileWorkers    int // параллельных сборок на реплику воркера
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type LoggingConfig struct {
	Level  string
	Format string
	Async  bool
}

type MetricsConfig struct {
	Enabled bool
	Port    int
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
	// CIDR прокси, по которым разбирается X-Forwarded-For. пусто - от соседа
	// из loopback и приватных сетей берётся только X-Real-IP. нужен и при
	// выключенном лимите (аудит, логи)
	TrustedProxies []string
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
	if c.Worker.Timeout <= 0 {
		return fmt.Errorf("WORKER_TIMEOUT must be positive")
	}

	if c.Executor.Timeout <= 0 {
		return fmt.Errorf("EXECUTOR_TIMEOUT must be positive")
	}
	// меньше 6 МБ docker контейнер не создаст, и упадёт каждый матч
	if c.Executor.MemoryLimit < minExecutorMemory {
		return fmt.Errorf("EXECUTOR_MEMORY_LIMIT must be at least %d bytes (got %d)", minExecutorMemory, c.Executor.MemoryLimit)
	}
	if c.Executor.DefaultIterations < 1 {
		return fmt.Errorf("EXECUTOR_DEFAULT_ITERATIONS must be positive")
	}
	// таймаут обработки матча накрывает таймаут контейнера с запасом на
	// create/cleanup и запись результата. иначе первым истекает ctx воркера,
	// таймаут программы не записывается и матч крутится через recovery вечно
	if c.Worker.Timeout < c.Executor.Timeout+workerTimeoutMargin {
		return fmt.Errorf("WORKER_TIMEOUT (%s) must be at least EXECUTOR_TIMEOUT (%s) + %s",
			c.Worker.Timeout, c.Executor.Timeout, workerTimeoutMargin)
	}
	if c.Executor.CompileWorkers < 1 {
		return fmt.Errorf("executor compile_workers must be positive")
	}

	// jwt проверяется строго только в prod
	// TODO: валидировать бы ещё format логгера, пока проверяется только level
	if err := validateJWTSecret(c.JWT.Secret, isProductionEnv()); err != nil {
		return err
	}
	if c.JWT.AccessTTL < 1*time.Minute {
		return fmt.Errorf("JWT access_ttl is too short")
	}

	for _, cidr := range c.RateLimit.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid TRUSTED_PROXIES entry %q: %w", cidr, err)
		}
	}

	validLevels := []string{"debug", "info", "warn", "error"}
	validLevel := slices.Contains(validLevels, c.Logging.Level)
	if !validLevel {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	return nil
}

// redisPoolReserve - соединения редиса сверх WORKER_MAX: публикация событий,
// кэш, автоскейлер, outbox, компиляция
const redisPoolReserve = 20

// пул подбирается под воркеров, но не больше дефолтного лимита постгреса (100).
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
	// .env подхватывается если есть, нет так нет
	_ = godotenv.Load()

	var env envReader

	// по умолчанию воркеров столько же, сколько ядер: каждый держит матч-контейнер
	// с квотой в ядро, больше - переподписка CPU и ложные таймауты программ.
	// от WORKER_MAX же считаются дефолты пулов бд и редиса
	workerMax := env.Int("WORKER_MAX", runtime.NumCPU())
	defaultPoolSize := recommendedDBPoolSize(workerMax)

	cfg := &Config{
		Server: ServerConfig{
			Port:            env.Int("API_PORT", 8080),
			ReadTimeout:     env.Duration("READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    env.Duration("WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second),
			BaseURL:         getEnv("BASE_URL", "http://localhost:8080"),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           env.Int("DB_PORT", 5432),
			User:           getEnv("DB_USER", "tjudge"),
			Password:       getEnvOrFile("DB_PASSWORD", "secret"),
			Name:           getEnv("DB_NAME", "tjudge"),
			SSLMode:        getEnv("DB_SSLMODE", "disable"),
			MaxConnections: env.Int("DB_MAX_CONNECTIONS", defaultPoolSize),
			MaxIdle:        env.Int("DB_MAX_IDLE", defaultPoolSize/5), // ~20% от макс
			MaxLifetime:    env.Duration("DB_MAX_LIFETIME", 1*time.Hour),

			PartitionRetentionMonths: env.Int("DB_PARTITION_RETENTION_MONTHS", 0),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     env.Int("REDIS_PORT", 6379),
			Password: getEnvOrFile("REDIS_PASSWORD", ""),
			DB:       env.Int("REDIS_DB", 0),
			// простаивающий воркер держит соединение на BRPOP, поэтому пул не
			// меньше WORKER_MAX плюс запас на остальные команды
			PoolSize: max(env.Int("REDIS_POOL_SIZE", 100), workerMax+redisPoolReserve),
		},
		Worker: WorkerConfig{
			MinWorkers:        env.Int("WORKER_MIN", min(2, workerMax)),
			MaxWorkers:        workerMax,
			Timeout:           env.Duration("WORKER_TIMEOUT", 90*time.Second),
			RetryAttempts:     env.Int("WORKER_RETRY_ATTEMPTS", 3),
			RetryDelay:        env.Duration("WORKER_RETRY_DELAY", 5*time.Second),
			AutoScaleInterval: env.Duration("WORKER_AUTOSCALE_INTERVAL", 2*time.Second),
		},
		Executor: ExecutorConfig{
			DockerImage:       getEnv("EXECUTOR_DOCKER_IMAGE", "tjudge-cli:latest"),
			Timeout:           env.Duration("EXECUTOR_TIMEOUT", 60*time.Second),
			CPUQuota:          int64(env.Int("EXECUTOR_CPU_QUOTA", 100000)),
			MemoryLimit:       int64(env.Int("EXECUTOR_MEMORY_LIMIT", 536870912)),
			PidsLimit:         int64(env.Int("EXECUTOR_PIDS_LIMIT", 100)),
			DefaultIterations: env.Int("EXECUTOR_DEFAULT_ITERATIONS", 100),
			Verbose:           env.Bool("EXECUTOR_VERBOSE", false),
			SeccompProfile:    getEnv("EXECUTOR_SECCOMP_PROFILE", ""),
			AppArmorProfile:   getEnv("EXECUTOR_APPARMOR_PROFILE", ""),
			BuilderImage:      getEnv("EXECUTOR_BUILDER_IMAGE", "tjudge-builder:latest"),
			CompileTimeout:    env.Duration("EXECUTOR_COMPILE_TIMEOUT", 120*time.Second),
			CompileWorkers:    env.Int("EXECUTOR_COMPILE_WORKERS", 2),
			CPUSetCPUs:        getEnv("EXECUTOR_CPUSET_CPUS", ""),
		},
		Storage: StorageConfig{
			ProgramsPath:     getEnv("PROGRAMS_PATH", "/data/programs"),
			HostProgramsPath: getEnv("HOST_PROGRAMS_PATH", ""), // пусто = берётся ProgramsPath
		},
		JWT: JWTConfig{
			Secret:     getEnvOrFile("JWT_SECRET", defaultJWTSecret),
			AccessTTL:  env.Duration("JWT_ACCESS_TTL", time.Hour),       // дальше тихий refresh
			RefreshTTL: env.Duration("JWT_REFRESH_TTL", 7*24*time.Hour), // неделя неактивности
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
			Async:  env.Bool("LOG_ASYNC", true), // в проде асинхронно
		},
		Metrics: MetricsConfig{
			Enabled: env.Bool("METRICS_ENABLED", true),
			Port:    env.Int("METRICS_PORT", 9090),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitAndTrim(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         env.Int("CORS_MAX_AGE", 3600),
		},
		RateLimit: RateLimitConfig{
			Enabled:           env.Bool("RATE_LIMIT_ENABLED", false), // в дев-режиме выключен
			RequestsPerMinute: env.Int("RATE_LIMIT_RPM", 100),
			TrustedProxies:    splitAndTrim(getEnv("TRUSTED_PROXIES", "")),
		},
	}

	// опечатка в .env не должна молча превращаться в дефолт
	if err := errors.Join(env.errs...); err != nil {
		return nil, fmt.Errorf("invalid environment: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// csv режется по запятой, пустые куски выкидываются
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

// envReader разбирает числовые, булевы и длительности из env и копит ошибки:
// RATE_LIMIT_ENABLED=yes, EXECUTOR_MEMORY_LIMIT=512m или WORKER_TIMEOUT=90
// раньше молча давали дефолт или мусор
type envReader struct {
	errs []error
}

func (e *envReader) Int(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s: invalid integer %q", key, value))
		return defaultValue
	}
	return result
}

func (e *envReader) Bool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s: invalid boolean %q", key, value))
		return defaultValue
	}
	return result
}

func (e *envReader) Duration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s: invalid duration %q (unit required, e.g. 90s)", key, value))
		return defaultValue
	}
	return result
}

// сначала берётся обычная переменная, потом KEY_FILE (docker secrets)
func getEnvOrFile(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	fileKey := key + "_FILE"
	if filePath := os.Getenv(fileKey); filePath != "" {
		content, err := os.ReadFile(filePath) // #nosec G304 -- путь из env, это docker secrets
		if err == nil {
			return strings.TrimSpace(string(content)) // убирается хвостовой перевод строки
		}
	}

	return defaultValue
}
