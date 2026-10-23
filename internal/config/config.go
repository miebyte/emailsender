package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/miebyte/emailsender/internal/smtp"
	"github.com/miebyte/goutils/mysqlutils"
	"github.com/miebyte/goutils/redisutils"
)

type Config struct {
	GRPCPort  int                     `json:"grpc_port"`
	MySQL     *mysqlutils.MysqlConfig `json:"mysql"`
	Redis     *redisutils.RedisConfig `json:"redis"`
	SMTP      *smtp.Config            `json:"smtp"`
	QueueName string                  `json:"queue_name"`
}

func (c *Config) SetDefault() {
	if c.QueueName == "" {
		c.QueueName = "emailsender:tasks"
	}
	if c.GRPCPort == 0 {
		c.GRPCPort = 28082
	}
	if c.MySQL != nil {
		if c.MySQL.Charset == "" {
			c.MySQL.Charset = "utf8mb4"
		}
		if c.MySQL.PoolSize == 0 {
			c.MySQL.PoolSize = 20
		}
	}
	if c.Redis != nil {
		if c.Redis.Timeout == 0 {
			c.Redis.Timeout = 2
		}
		c.Redis.SetDefault()
	}
	if c.SMTP != nil {
		c.SMTP.SetDefault()
	}
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.QueueName) == "" {
		return fmt.Errorf("queue_name is required")
	}
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("grpc_port must be between 1 and 65535")
	}
	if c.MySQL == nil {
		return fmt.Errorf("mysql config is required")
	}
	if err := validateAddress(c.MySQL.Instance); err != nil {
		return fmt.Errorf("mysql.instance: %w", err)
	}
	if strings.TrimSpace(c.MySQL.Database) == "" || strings.TrimSpace(c.MySQL.Username) == "" {
		return fmt.Errorf("mysql database and username are required")
	}
	if c.MySQL.PoolSize < 1 {
		return fmt.Errorf("mysql.pool_size must be positive")
	}
	if c.Redis == nil {
		return fmt.Errorf("redis config is required")
	}
	if err := validateAddress(c.Redis.Server); err != nil {
		return fmt.Errorf("redis.server: %w", err)
	}
	if c.Redis.Db < 0 || c.Redis.PoolSize < 1 || c.Redis.MinSize < 0 || c.Redis.MaxSize < c.Redis.MinSize || c.Redis.MaxSize > c.Redis.PoolSize {
		return fmt.Errorf("redis database or pool sizes are invalid")
	}
	if c.Redis.Timeout < 2 || c.Redis.Timeout > 60 {
		return fmt.Errorf("redis.timeout must be between 2 and 60 seconds")
	}
	if !c.Redis.ContextTimeoutEnabled || c.Redis.MaxRetries != -1 {
		return fmt.Errorf("email queue requires redis.context_timeout_enabled=true and redis.max_retries=-1")
	}
	if c.SMTP == nil {
		return fmt.Errorf("smtp config is required")
	}
	if err := c.SMTP.Validate(); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	return nil
}

func (c *Config) GrpcAddr() string {
	return fmt.Sprintf(":%d", c.GRPCPort)
}

func validateAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("expected host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
