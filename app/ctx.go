// File:		ctx.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package app

import (
	"github.com/go-puzzles/emailsender/domain/email"
	"github.com/go-puzzles/emailsender/domain/event"
	"github.com/go-puzzles/emailsender/domain/record"
	"github.com/go-puzzles/emailsender/pkg/dal/base"
	"github.com/go-puzzles/emailsender/pkg/eventbus"
	"github.com/go-puzzles/puzzles/goredis"
	"gorm.io/gorm"

	recordDal "github.com/go-puzzles/emailsender/pkg/dal/record"
	emailSrv "github.com/go-puzzles/emailsender/service/email"
	recordSrv "github.com/go-puzzles/emailsender/service/record"
)

type AppContext struct {
	MysqlDb *base.BaseDal
	RedisDb *goredis.PuzzleRedisClient

	EventBus event.EventBus

	emailConf *email.Config
	EmailSrv  email.Service

	RecordSrv record.Service
}

func NewAppContext(
	db *gorm.DB,
	rdb *goredis.PuzzleRedisClient,
	emailConf *email.Config,
) *AppContext {
	eventBus := eventbus.NewMemoryEventBus()

	baseDal := base.NewBaseDal(db)
	recordRepo := recordDal.NewRecordRepo(baseDal)

	recordService := recordSrv.NewRecordService(recordRepo)
	emailService := emailSrv.NewEmailService(emailConf, eventBus, rdb)

	registerRecordEvent(eventBus, recordService)

	return &AppContext{
		MysqlDb:   baseDal,
		RedisDb:   rdb,
		emailConf: emailConf,
		EmailSrv:  emailService,
		RecordSrv: recordService,
		EventBus:  eventBus,
	}
}

func registerRecordEvent(eventBus event.EventBus, eventHandler record.Event) {
	eventBus.Subscribe(record.EmailStatusEvent{}.EventName(), eventHandler.UpdateRecordStatusEvent)
}
