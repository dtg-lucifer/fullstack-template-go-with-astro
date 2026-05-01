// Package config loads and validates application configuration from config.yaml.
// Non-secret, static settings live in config.yaml; secrets and per-environment
// overrides live in .env (or real environment variables in production).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Top-level ─────────────────────────────────────────────────────────────────

// Config is the root structure populated from config.yaml.
type Config struct {
	Server      Server      `yaml:"server"`
	Security    Security    `yaml:"security"`
	Database    Database    `yaml:"database"`
	Redis       Redis       `yaml:"redis"`
	Logging     Logging     `yaml:"logging"`
	Middlewares Middlewares `yaml:"middlewares"`
	Realtime    Realtime    `yaml:"realtime"`
	Queue       Queue       `yaml:"queue"`
	Workers     Workers     `yaml:"workers"`
}

// ── Server ────────────────────────────────────────────────────────────────────

type Server struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	APIPrefix   string `yaml:"api_prefix"`
	Environment string `yaml:"environment"` // development | staging | production
	TLS         TLS    `yaml:"tls"`
}

type TLS struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// ── Security ──────────────────────────────────────────────────────────────────

type Security struct {
	Helmet    Helmet    `yaml:"helmet"`
	CORS      CORS      `yaml:"cors"`
	RateLimit RateLimit `yaml:"rate_limit"`
}

type Helmet struct {
	Enabled               bool `yaml:"enabled"`
	ContentSecurityPolicy bool `yaml:"content_security_policy"`
}

type CORS struct {
	Enabled bool     `yaml:"enabled"`
	Origins []string `yaml:"origins"`
}

type RateLimit struct {
	Enabled       bool `yaml:"enabled"`
	WindowSeconds int  `yaml:"window_seconds"` // sliding window length in seconds
	MaxRequests   int  `yaml:"max_requests"`
	SkipLocalhost bool `yaml:"skip_localhost"`
}

// ── Database ──────────────────────────────────────────────────────────────────

type Database struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	User              string `yaml:"user"`
	Password          string `yaml:"password"`
	Name              string `yaml:"name"`
	SSLMode           string `yaml:"ssl_mode"`
	PoolSize          int    `yaml:"pool_size"`
	ConnectionTimeout int    `yaml:"connection_timeout_ms"` // milliseconds
	IdleTimeout       int    `yaml:"idle_timeout_ms"`       // milliseconds
}

// ── Logging ───────────────────────────────────────────────────────────────────

type Logging struct {
	// Level controls the minimum level emitted to the console.
	// Valid: "debug" | "info" | "warn" | "error"
	// File transports always capture everything regardless of this setting.
	Level        string `yaml:"level"`
	Format       string `yaml:"format"` // "json" | "text"
	EnableColors bool   `yaml:"enable_colors"`
	LogRequests  bool   `yaml:"log_requests"`
	LogErrors    bool   `yaml:"log_errors"`
}

// ── Middlewares ───────────────────────────────────────────────────────────────

type Middlewares struct {
	RequestID           RequestIDMiddleware           `yaml:"request_id"`
	BodyParser          BodyParserMiddleware          `yaml:"body_parser"`
	DependencyInjection DependencyInjectionMiddleware `yaml:"dependency_injection"`
	Logger              LoggerMiddlewareConfig        `yaml:"logger"`
}

type RequestIDMiddleware struct {
	Enabled    bool   `yaml:"enabled"`
	HeaderName string `yaml:"header_name"`
}

type BodyParserMiddleware struct {
	Enabled          bool  `yaml:"enabled"`
	JSONLimitBytes   int64 `yaml:"json_limit_bytes"`
	MaxBodySizeBytes int64 `yaml:"max_body_size_bytes"`
}

type DependencyInjectionMiddleware struct {
	Enabled bool `yaml:"enabled"`
}

type LoggerMiddlewareConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ── Realtime (WebSocket) ──────────────────────────────────────────────────────

type Realtime struct {
	WebSocket WebSocketConfig `yaml:"websocket"`
}

type WebSocketConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Path            string `yaml:"path"` // e.g. "/ws"
	ReadBufferSize  int    `yaml:"read_buffer_size"`
	WriteBufferSize int    `yaml:"write_buffer_size"`
}

// ── Redis ─────────────────────────────────────────────────────────────────────

type Redis struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// ── Queue (RabbitMQ) ──────────────────────────────────────────────────────────

type Queue struct {
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
}
type RabbitMQConfig struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultAttempts int    `yaml:"default_attempts"` // retry count on failure
	DefaultBackoff  string `yaml:"default_backoff"`  // e.g. "1s", "500ms"
}

// ── Workers ───────────────────────────────────────────────────────────────────

type Workers struct {
	Process          WorkerProcess          `yaml:"process"`
	NotificationJobs NotificationJobsWorker `yaml:"notification_jobs"`
}

type WorkerProcess struct {
	Enabled bool `yaml:"enabled"`
}

type NotificationJobsWorker struct {
	Enabled     bool `yaml:"enabled"`
	Concurrency int  `yaml:"concurrency"`
}

// ── Loader ────────────────────────────────────────────────────────────────────

// NewConfig reads the YAML file at path, unmarshals it, applies defaults, and
// then applies environment variable overrides. If path is empty it defaults to
// "config.yaml".
func NewConfig(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(cfg)
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyDefaults fills in sensible values for any field left at its zero value.
func applyDefaults(cfg *Config) {
	if cfg.Server.APIPrefix == "" {
		cfg.Server.APIPrefix = "/api/v1"
	}
	if cfg.Server.Environment == "" {
		cfg.Server.Environment = "development"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.PoolSize == 0 {
		cfg.Database.PoolSize = 10
	}
	if cfg.Database.ConnectionTimeout == 0 {
		cfg.Database.ConnectionTimeout = 10000
	}
	if cfg.Database.IdleTimeout == 0 {
		cfg.Database.IdleTimeout = 30000
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "text"
	}
	if cfg.Middlewares.RequestID.HeaderName == "" {
		cfg.Middlewares.RequestID.HeaderName = "X-Request-ID"
	}
	if cfg.Middlewares.BodyParser.JSONLimitBytes == 0 {
		cfg.Middlewares.BodyParser.JSONLimitBytes = 10 * 1024 * 1024 // 10 MB
	}
	if cfg.Realtime.WebSocket.Path == "" {
		cfg.Realtime.WebSocket.Path = "/ws"
	}
	if cfg.Realtime.WebSocket.ReadBufferSize == 0 {
		cfg.Realtime.WebSocket.ReadBufferSize = 1024
	}
	if cfg.Realtime.WebSocket.WriteBufferSize == 0 {
		cfg.Realtime.WebSocket.WriteBufferSize = 1024
	}
	if cfg.Queue.RabbitMQ.DefaultAttempts == 0 {
		cfg.Queue.RabbitMQ.DefaultAttempts = 3
	}
	if cfg.Queue.RabbitMQ.DefaultBackoff == "" {
		cfg.Queue.RabbitMQ.DefaultBackoff = "1s"
	}
	if cfg.Workers.NotificationJobs.Concurrency == 0 {
		cfg.Workers.NotificationJobs.Concurrency = 5
	}
	if cfg.Security.RateLimit.WindowSeconds == 0 {
		cfg.Security.RateLimit.WindowSeconds = 900
	}
	if cfg.Security.RateLimit.MaxRequests == 0 {
		cfg.Security.RateLimit.MaxRequests = 100
	}
	if cfg.Redis.Host == "" {
		cfg.Redis.Host = "localhost"
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 10
	}
}

// applyEnvOverrides lets environment variables override config.yaml values.
// This mirrors the ConfigManager.applyEnvironmentOverrides() pattern from the
// Express template.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("ENV"); v != "" {
		cfg.Server.Environment = v
	}
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		cfg.Security.CORS.Origins = splitTrim(v, ",")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// DSN builds a PostgreSQL connection string.
// DB_URL env var takes full precedence over individual fields.
func (c *Config) DSN() string {
	if url := os.Getenv("DB_URL"); url != "" {
		return url
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Database.User, c.Database.Password,
		c.Database.Host, c.Database.Port,
		c.Database.Name, c.Database.SSLMode,
	)
}

// Addr returns the "host:port" string the HTTP server should listen on.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// IsProduction returns true when the environment is "production".
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

// AMQPUrl returns the RabbitMQ connection URL from the AMQP_URL env var,
// falling back to a local default.
func (c *Config) AMQPUrl() string {
	if url := os.Getenv("AMQP_URL"); url != "" {
		return url
	}
	return "amqp://guest:guest@localhost:5672/"
}

// RedisAddr returns the "host:port" Redis address.
// REDIS_URL env var (in "host:port" form) takes precedence over config.yaml fields.
func (c *Config) RedisAddr() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}

// RedisPassword returns the Redis password from the REDIS_PASSWORD env var,
// falling back to the config.yaml value.
func (c *Config) RedisPassword() string {
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		return v
	}
	return c.Redis.Password
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
