// File:		ctx.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package app

import (
	"gitea.hoven.com/core/emailsender/domain/email"
	"gitea.hoven.com/core/emailsender/domain/event"
	"gitea.hoven.com/core/emailsender/domain/record"
	"gitea.hoven.com/core/emailsender/pkg/dal/base"
	"gitea.hoven.com/core/emailsender/pkg/eventbus"
	"github.com/go-puzzles/puzzles/goredis"
	"gorm.io/gorm"

	recordDal "gitea.hoven.com/core/emailsender/pkg/dal/record"
	emailSrv "gitea.hoven.com/core/emailsender/service/email"
	recordSrv "gitea.hoven.com/core/emailsender/service/record"
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
