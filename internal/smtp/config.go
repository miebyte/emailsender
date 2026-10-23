package smtp

import (
	"errors"
	"net/mail"
	"strings"
)

type Config struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func (c *Config) SetDefault() {
	if c.Port == 0 {
		c.Port = 465
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 30
	}
}

func (c *Config) Validate() error {
	if c.Host == "" || strings.ContainsAny(c.Host, " \t\r\n/") {
		return errors.New("invalid smtp server")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("invalid smtp port")
	}
	if strings.ContainsAny(c.Username, "\r\n") {
		return errors.New("invalid smtp username")
	}
	if _, err := mail.ParseAddress(c.Username); err != nil {
		return errors.New("invalid smtp username")
	}
	if c.Password == "" {
		return errors.New("missing smtp password")
	}
	if c.TimeoutSeconds <= 0 {
		return errors.New("smtp timeout_seconds must be positive")
	}
	return nil
}
