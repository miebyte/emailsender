// File:		ctx.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package app

import (
	"github.com/miebyte/emailsender/domain/email"
	"github.com/miebyte/emailsender/domain/event"
	"github.com/miebyte/emailsender/domain/record"
	"github.com/miebyte/emailsender/pkg/dal/base"
	"github.com/miebyte/emailsender/pkg/eventbus"
	"github.com/miebyte/goutils/redisutils"
	"gorm.io/gorm"

	recordDal "github.com/miebyte/emailsender/pkg/dal/record"
	emailSrv "github.com/miebyte/emailsender/service/email"
	recordSrv "github.com/miebyte/emailsender/service/record"
)

type AppContext struct {
	MysqlDb   *base.BaseDal
	RedisDb   *redisutils.RedisClient
	EventBus  event.EventBus
	emailConf *email.Config
	EmailSrv  email.Service
	RecordSrv record.Service
}

func NewAppContext(
	db *gorm.DB,
	rdb *redisutils.RedisClient,
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
