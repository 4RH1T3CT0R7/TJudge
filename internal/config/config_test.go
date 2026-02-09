package config

import (
	"os"
	"path/filepath"
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
		Worker:   WorkerConfig{MinWorkers: 1, MaxWorkers: 10, QueueSize: 100},
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

func TestConfig_Validate_WorkerQueueSizeLessThan1(t *testing.T) {
	cfg := validConfig()
	cfg.Worker.QueueSize = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue_size")
}

func TestConfig_Validate_JWTSecretInProduction(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = "change-this-secret-in-production"
	t.Setenv("ENVIRONMENT", "production")
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret")
}

func TestConfig_Validate_JWTSecretInDev(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.Secret = "change-this-secret-in-production"
	t.Setenv("ENVIRONMENT", "development")
	// Should NOT error in non-production
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

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_INT", "42")
	assert.Equal(t, 42, getEnvInt("TEST_INT", 0))

	t.Setenv("TEST_INT_INVALID", "not-a-number")
	assert.Equal(t, 99, getEnvInt("TEST_INT_INVALID", 99))

	assert.Equal(t, 10, getEnvInt("NONEXISTENT_KEY_12345", 10))
}

func TestGetEnvBool(t *testing.T) {
	t.Setenv("TEST_BOOL_TRUE", "true")
	assert.True(t, getEnvBool("TEST_BOOL_TRUE", false))

	t.Setenv("TEST_BOOL_ONE", "1")
	assert.True(t, getEnvBool("TEST_BOOL_ONE", false))

	t.Setenv("TEST_BOOL_FALSE", "false")
	assert.False(t, getEnvBool("TEST_BOOL_FALSE", true))

	assert.True(t, getEnvBool("NONEXISTENT_KEY_12345", true))
}

func TestGetEnvDuration(t *testing.T) {
	t.Setenv("TEST_DUR", "5s")
	assert.Equal(t, 5*time.Second, getEnvDuration("TEST_DUR", time.Minute))

	t.Setenv("TEST_DUR_INVALID", "not-duration")
	assert.Equal(t, time.Minute, getEnvDuration("TEST_DUR_INVALID", time.Minute))

	assert.Equal(t, 10*time.Second, getEnvDuration("NONEXISTENT_KEY_12345", 10*time.Second))
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
		Password: "secret",
		Name:     "tjudge",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=tjudge")
	assert.Contains(t, dsn, "password=secret")
	assert.Contains(t, dsn, "dbname=tjudge")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestDatabaseConfig_DSNURL(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "tjudge",
		Password: "secret",
		Name:     "tjudge",
		SSLMode:  "disable",
	}
	url := cfg.DSNURL()
	assert.Equal(t, "postgres://tjudge:secret@localhost:5432/tjudge?sslmode=disable", url)
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
		"REDIS_HOST", "REDIS_PORT", "WORKER_MIN", "WORKER_MAX", "WORKER_QUEUE_SIZE",
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
	t.Setenv("WORKER_QUEUE_SIZE", "500")
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
	assert.Equal(t, 500, cfg.Worker.QueueSize)
	assert.Equal(t, 30*time.Minute, cfg.JWT.AccessTTL)
}
