package config

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Validate() ---

func validConfig() *Config {
	return &Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Host: "localhost", Port: 5432, User: "tjudge", Name: "tjudge", MaxConnections: 10},
		Redis:    RedisConfig{Host: "localhost", Port: 6379},
		Worker:   WorkerConfig{MinWorkers: 1, MaxWorkers: 10, Timeout: 90 * time.Second},
		Executor: ExecutorConfig{Timeout: time.Minute, MemoryLimit: 512 << 20, DefaultIterations: 100, CompileWorkers: 2},
		JWT:      JWTConfig{Secret: "test-secret-minimum-length", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour},
		Logging:  LoggingConfig{Level: "info", Format: "json"},
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := validConfig()
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_InvalidServerPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 65536},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Server.Port = tc.port
			err := cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "server port")
		})
	}
}

func TestConfig_Validate_EmptyDBHost(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Host = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database host")
}

func TestConfig_Validate_EmptyDBUser(t *testing.T) {
	cfg := validConfig()
	cfg.Database.User = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database user")
}

func TestConfig_Validate_EmptyDBName(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Name = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database name")
}

func TestConfig_Validate_MaxConnectionsLessThan1(t *testing.T) {
	cfg := validConfig()
	cfg.Database.MaxConnections = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_connections")
}

func TestConfig_Validate_EmptyRedisHost(t *testing.T) {
	cfg := validConfig()
	cfg.Redis.Host = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis host")
}

func TestConfig_Validate_InvalidRedisPort(t *testing.T) {
	cfg := validConfig()
	cfg.Redis.Port = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis port")
}

func TestConfig_Validate_WorkerMinLessThan1(t *testing.T) {
	cfg := validConfig()
	cfg.Worker.MinWorkers = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_workers")
}

func TestConfig_Validate_WorkerMaxLessThanMin(t *testing.T) {
	cfg := validConfig()
	cfg.Worker.MinWorkers = 5
	cfg.Worker.MaxWorkers = 3
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_workers")
}

// лимит памяти ниже докерного минимума и пустые таймауты ловятся на старте,
// а не падением каждого матча
func TestConfig_Validate_Executor(t *testing.T) {
	cfg := validConfig()
	cfg.Executor.MemoryLimit = 512 // "512m" раньше парсилось как 512 байт
	assert.ErrorContains(t, cfg.Validate(), "EXECUTOR_MEMORY_LIMIT")

	cfg = validConfig()
	cfg.Executor.Timeout = 0
	assert.ErrorContains(t, cfg.Validate(), "EXECUTOR_TIMEOUT")

	cfg = validConfig()
	cfg.Executor.DefaultIterations = 0
	assert.ErrorContains(t, cfg.Validate(), "EXECUTOR_DEFAULT_ITERATIONS")
}

// ctx воркера не должен истекать раньше таймаута контейнера: иначе таймаут
// программы не записывается и матч бесконечно возвращается через recovery
func TestConfig_Validate_WorkerTimeoutCoversExecutor(t *testing.T) {
	cfg := validConfig()
	cfg.Worker.Timeout = 60 * time.Second
	cfg.Executor.Timeout = 60 * time.Second
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WORKER_TIMEOUT")

	cfg.Worker.Timeout = 80 * time.Second
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_CompileWorkersLessThan1(t *testing.T) {
	cfg := validConfig()
	cfg.Executor.CompileWorkers = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compile_workers")
}

func TestConfig_Validate_JWTSecretInProduction(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = "change-this-secret-in-production"
	t.Setenv("ENVIRONMENT", "production")
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestConfig_Validate_JWTSecretInDev(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = "change-this-secret-in-production"
	t.Setenv("ENVIRONMENT", "development")
	// Should NOT error in non-production
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_JWTSecretPlaceholderBlacklist(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	placeholders := []string{
		"CHANGE_ME_TO_STRONG_RANDOM_SECRET_IN_PRODUCTION",
		"changeme",
		"CHANGE-ME",
		"secret",
		"your-secret-key-change-in-production",
	}
	for _, ph := range placeholders {
		t.Run(ph, func(t *testing.T) {
			cfg := validConfig()
			cfg.JWT.Secret = ph
			err := cfg.Validate()
			assert.Error(t, err, "placeholder %q must be rejected", ph)
			assert.Contains(t, err.Error(), "JWT_SECRET")
		})
	}
}

func TestConfig_Validate_JWTSecretTooShortInProd(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = "short-secret"
	t.Setenv("ENVIRONMENT", "production")
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least")
}

func TestConfig_Validate_JWTSecretEmptyInProd(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = ""
	t.Setenv("ENVIRONMENT", "production")
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestConfig_Validate_JWTSecretValidInProd(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = "a-real-random-secret-at-least-32-bytes-long-1234"
	t.Setenv("ENVIRONMENT", "production")
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_JWTSecretPlaceholderInDevOK(t *testing.T) {
	// Placeholders acceptable in dev (with warning in practice)
	cfg := validConfig()
	cfg.JWT.Secret = "CHANGE_ME"
	t.Setenv("ENVIRONMENT", "development")
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_AccessTTLTooShort(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.AccessTTL = 30 * time.Second
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access_ttl")
}

func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	cfg := validConfig()
	cfg.Logging.Level = "trace"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging level")
}

func TestConfig_Validate_AllValidLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		t.Run(level, func(t *testing.T) {
			cfg := validConfig()
			cfg.Logging.Level = level
			assert.NoError(t, cfg.Validate())
		})
	}
}

// --- Helper functions ---

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_KEY", "value")
	assert.Equal(t, "value", getEnv("TEST_KEY", "default"))
	assert.Equal(t, "default", getEnv("NONEXISTENT_KEY_12345", "default"))
}

func TestEnvReader(t *testing.T) {
	var env envReader

	t.Setenv("TEST_INT", " 42 ")
	t.Setenv("TEST_BOOL", "True")
	t.Setenv("TEST_DUR", "5s")
	assert.Equal(t, 42, env.Int("TEST_INT", 0))
	assert.True(t, env.Bool("TEST_BOOL", false))
	assert.Equal(t, 5*time.Second, env.Duration("TEST_DUR", time.Minute))
	assert.Equal(t, 10, env.Int("NONEXISTENT_KEY_12345", 10))
	assert.Empty(t, env.errs)

	// опечатки копятся как ошибки, а не превращаются молча в дефолт
	t.Setenv("TEST_INT_BAD", "512m")
	t.Setenv("TEST_BOOL_BAD", "yes")
	t.Setenv("TEST_DUR_BAD", "90")
	assert.Equal(t, 99, env.Int("TEST_INT_BAD", 99))
	assert.False(t, env.Bool("TEST_BOOL_BAD", false))
	assert.Equal(t, time.Minute, env.Duration("TEST_DUR_BAD", time.Minute))
	assert.Len(t, env.errs, 3)
}

func TestLoad_InvalidEnvFails(t *testing.T) {
	clearEnvKeys(t)
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "tjudge")
	t.Setenv("DB_NAME", "tjudge")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("RATE_LIMIT_ENABLED", "yes")

	_, err := Load()
	assert.ErrorContains(t, err, "RATE_LIMIT_ENABLED")
}

func TestGetEnvOrFile(t *testing.T) {
	// Direct env var
	t.Setenv("TEST_SECRET", "direct-value")
	assert.Equal(t, "direct-value", getEnvOrFile("TEST_SECRET", "default"))

	// From file
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	err := os.WriteFile(secretFile, []byte("file-secret\n"), 0600)
	require.NoError(t, err)

	// Clear direct var, set file var
	t.Setenv("TEST_FILE_SECRET", "")
	t.Setenv("TEST_FILE_SECRET_FILE", secretFile)
	result := getEnvOrFile("TEST_FILE_SECRET", "default")
	assert.Equal(t, "file-secret", result)

	// Default when neither exists
	assert.Equal(t, "default", getEnvOrFile("NONEXISTENT_KEY_12345", "default"))
}

// --- DSN/DSNURL/Address ---

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "tjudge",
		Password: `it's a \secret`,
		Name:     "tjudge",
		SSLMode:  "disable",
	}
	assert.Equal(t,
		`host='localhost' port=5432 user='tjudge' password='it\'s a \\secret' dbname='tjudge' sslmode='disable' timezone=UTC`,
		cfg.DSN())
}

func TestDatabaseConfig_DSNURL(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "tjudge",
		Password: "p@ss/w#rd x",
		Name:     "tjudge",
		SSLMode:  "disable",
	}
	dsn := cfg.DSNURL()
	assert.Equal(t, "postgres://tjudge:p%40ss%2Fw%23rd%20x@localhost:5432/tjudge?sslmode=disable&timezone=UTC", dsn)

	// пароль со спецсимволами должен разбираться обратно без потерь
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	password, _ := u.User.Password()
	assert.Equal(t, cfg.Password, password)
}

func TestRedisConfig_Address(t *testing.T) {
	cfg := RedisConfig{Host: "localhost", Port: 6379}
	assert.Equal(t, "localhost:6379", cfg.Address())
}

// --- Load() with env overrides ---

func TestLoad_DefaultValues(t *testing.T) {
	// Clear all env vars that Load reads so defaults are used
	envVars := []string{
		"API_PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"REDIS_HOST", "REDIS_PORT", "WORKER_MIN", "WORKER_MAX",
		"JWT_SECRET", "JWT_ACCESS_TTL", "LOG_LEVEL", "ENVIRONMENT",
	}
	for _, key := range envVars {
		t.Setenv(key, "")
	}
	// Ensure Load doesn't pick up .env file by setting key defaults
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "tjudge")
	t.Setenv("DB_NAME", "tjudge")
	t.Setenv("REDIS_HOST", "localhost")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("API_PORT", "9090")
	t.Setenv("DB_HOST", "dbhost")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "myuser")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("WORKER_MIN", "2")
	t.Setenv("WORKER_MAX", "20")
	t.Setenv("JWT_ACCESS_TTL", "30m")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "dbhost", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "myuser", cfg.Database.User)
	assert.Equal(t, "mydb", cfg.Database.Name)
	assert.Equal(t, "redis", cfg.Redis.Host)
	assert.Equal(t, 6380, cfg.Redis.Port)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, 2, cfg.Worker.MinWorkers)
	assert.Equal(t, 20, cfg.Worker.MaxWorkers)
	assert.Equal(t, 30*time.Minute, cfg.JWT.AccessTTL)
}

// по умолчанию пул воркеров по числу ядер, а пул редиса не меньше WORKER_MAX
// с запасом: простаивающие воркеры держат соединения на BRPOP
func TestLoad_WorkerAndRedisPoolDefaults(t *testing.T) {
	clearEnvKeys(t)
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "tjudge")
	t.Setenv("DB_NAME", "tjudge")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_POOL_SIZE", "")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, runtime.NumCPU(), cfg.Worker.MaxWorkers)
	assert.LessOrEqual(t, cfg.Worker.MinWorkers, cfg.Worker.MaxWorkers)
	assert.GreaterOrEqual(t, cfg.Redis.PoolSize, cfg.Worker.MaxWorkers+redisPoolReserve)

	// явный пул меньше WORKER_MAX поднимается до WORKER_MAX + запас
	t.Setenv("WORKER_MAX", "200")
	t.Setenv("REDIS_POOL_SIZE", "100")
	cfg, err = Load()
	require.NoError(t, err)
	assert.Equal(t, 200+redisPoolReserve, cfg.Redis.PoolSize)
}

// TestRecommendedDBPoolSize проверяет формулу recommendedDBPoolSize.
func TestRecommendedDBPoolSize(t *testing.T) {
	cases := []struct {
		workerMax int
		want      int
		label     string
	}{
		{0, 20, "min clamp когда worker=0"},
		{10, 35, "10 workers * 1.5 + 20 = 35"},
		{50, 95, "50 workers * 1.5 + 20 = 95"},
		{100, 100, "ceiling=100"},
		{1000, 100, "ceiling=100 даже при большом WORKER_MAX"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			got := recommendedDBPoolSize(c.workerMax)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestLoad_DBPoolAutoSized проверяет, что по умолчанию DB_MAX_CONNECTIONS
// подстраивается под WORKER_MAX.
func TestLoad_DBPoolAutoSized(t *testing.T) {
	clearEnvKeys(t)
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "tjudge")
	t.Setenv("DB_NAME", "tjudge")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("WORKER_MAX", "30")
	// DB_MAX_CONNECTIONS не задан, должно быть 30*1.5+20 = 65.
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 65, cfg.Database.MaxConnections)
	assert.Equal(t, 13, cfg.Database.MaxIdle, "idle = 20% от max")
}

// clearEnvKeys - вспомогательная очистка всех env, которые подхватывает Load().
func clearEnvKeys(t *testing.T) {
	t.Helper()
	keys := []string{
		"API_PORT", "DB_PORT", "DB_HOST", "DB_USER", "DB_NAME",
		"DB_MAX_CONNECTIONS", "DB_MAX_IDLE",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD",
		"WORKER_MIN", "WORKER_MAX", "RATE_LIMIT_ENABLED",
		"JWT_SECRET", "JWT_ACCESS_TTL", "LOG_LEVEL", "ENVIRONMENT",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}
}
