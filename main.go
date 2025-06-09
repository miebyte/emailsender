// File:		main.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"github.com/miebyte/emailsender/app"
	"github.com/miebyte/emailsender/domain/email"
	"github.com/miebyte/emailsender/pkg/dal/model"
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/mysqlutils"
	"github.com/miebyte/goutils/redisutils"

	emailSenderGrpc "github.com/miebyte/emailsender/api/grpc"
)

var (
	port            = flags.Int("port", 28080, "port to listen on")
	mysqlConfigFlag = flags.Struct("mysql", (*mysqlutils.MysqlConfig)(nil), "mysql config")
	redisConfFlag   = flags.Struct("redis", (*redisutils.RedisConfig)(nil), "redis auth config")
	emailConfFlag   = flags.Struct("email", (*email.Config)(nil), "email smtp config")
)

func main() {
	flags.Parse()

	mysqlConf := new(mysqlutils.MysqlConfig)
	logging.PanicError(mysqlConfigFlag(mysqlConf))

	emailConf := new(email.Config)
	logging.PanicError(emailConfFlag(emailConf))

	redisConf := new(redisutils.RedisConfig)
	logging.PanicError(redisConfFlag(redisConf))

	db, err := mysqlConf.DialMysqlGorm()
	logging.PanicError(err)
	logging.PanicError(db.AutoMigrate(model.AllTables()...))

	redisClient, err := redisConf.DialGORedisClient()
	logging.PanicError(err)

	emailSendApp := app.NewEmailSenderApp(app.NewAppContext(db, redisClient, emailConf))
	g := emailSenderGrpc.SetupGrpcServer(emailSendApp)

	srv := cores.NewCores(
		cores.WithWorker(emailSendApp.RunEmailAsyncTask),
		cores.WithGrpcServer(g),
		cores.WithGrpcUI(),
	)

	logging.PanicError(cores.Start(srv, port()))
}
