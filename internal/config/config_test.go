package config

import (
	"encoding/json"
	"testing"
)

func validConfig(t *testing.T) *Config {
	t.Helper()
	var cfg Config
	// Configuration parsing and validation must not contact external services.
	err := json.Unmarshal([]byte(`{
		"mysql":{"instance":"127.0.0.1:3306","database":"emailsender","username":"sender"},
		"redis":{"server":"127.0.0.1:6379","context_timeout_enabled":true,"max_retries":-1},
		"smtp":{"host":"smtp.example.com","port":465,"username":"sender@example.com","password":"example"}
	}`), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SetDefault()
	return &cfg
}

func TestDefaultsAndConfigFormat(t *testing.T) {
	cfg := validConfig(t)
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.GrpcAddr() != ":28082" || cfg.MySQL.Charset != "utf8mb4" || cfg.SMTP.TimeoutSeconds != 30 || cfg.Redis.Timeout != 2 {
		t.Fatalf("defaults not applied: grpc=%s charset=%s smtp timeout=%d redis timeout=%d", cfg.GrpcAddr(), cfg.MySQL.Charset, cfg.SMTP.TimeoutSeconds, cfg.Redis.Timeout)
	}
}

func TestInvalidConfigRejectedBeforeStartup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"missing mysql", func(c *Config) { c.MySQL = nil }},
		{"missing redis", func(c *Config) { c.Redis = nil }},
		{"missing email", func(c *Config) { c.SMTP = nil }},
		{"invalid grpc port", func(c *Config) { c.GRPCPort = 65536 }},
		{"invalid mysql address", func(c *Config) { c.MySQL.Instance = "localhost" }},
		{"missing database", func(c *Config) { c.MySQL.Database = "" }},
		{"invalid redis port", func(c *Config) { c.Redis.Server = "localhost:0" }},
		{"invalid redis pool", func(c *Config) { c.Redis.MaxSize = c.Redis.PoolSize + 1 }},
		{"invalid redis timeout", func(c *Config) { c.Redis.Timeout = -1 }},
		{"disabled redis context deadlines", func(c *Config) { c.Redis.ContextTimeoutEnabled = false }},
		{"enabled redis command retries", func(c *Config) { c.Redis.MaxRetries = 3 }},
		{"invalid SMTP port", func(c *Config) { c.SMTP.Port = 65536 }},
		{"missing queue", func(c *Config) { c.QueueName = " " }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(t)
			tc.change(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}

	var empty Config
	empty.SetDefault()
	if err := empty.Validate(); err == nil {
		t.Fatal("missing config accepted")
	}
}
