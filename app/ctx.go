// File:		ctx.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package app

import (
	"gitea.hoven.com/core/emailsender/pkg/email"
	"github.com/go-puzzles/puzzles/goredis"
)

type AppContext struct {
	EmailSender email.EmailSender
	RedisClient *goredis.PuzzleRedisClient
}

func NewAppContext(es email.EmailSender, rdb *goredis.PuzzleRedisClient) *AppContext {
	return &AppContext{
		EmailSender: es,
		RedisClient: rdb,
	}
}
