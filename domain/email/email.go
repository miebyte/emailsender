// File:		email.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package email

import (
	"errors"
	"fmt"
)

type AsyncEmailTask struct {
	Id          int
	Subject     string
	FromEmail   string
	TargetEmail string
	Msg         string
}

func (t *AsyncEmailTask) Key() string {
	return fmt.Sprintf("AsyncEmailTask:%v:%v", t.Id, t.TargetEmail)
}

type Config struct {
	SmtpServer string
	SmtpPort   string
	Identifier string
	Credential string
	AsyncQueue string
}

func (c *Config) Validate() error {
	if c.SmtpServer == "" {
		return errors.New("missing smtp server")
	}

	if c.SmtpPort == "" {
		return errors.New("missing smtp port")
	}

	if c.Identifier == "" {
		return errors.New("missing identifier")
	}

	if c.Credential == "" {
		return errors.New("missing credential")
	}

	if c.AsyncQueue == "" {
		return errors.New("missing async queue")
	}

	return nil
}

func (c *Config) SmtpUrl() string {
	return c.SmtpServer + ":" + c.SmtpPort
}
