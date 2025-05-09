// File:		main.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"github.com/go-puzzles/emailsender/app"
	"github.com/go-puzzles/emailsender/domain/email"
	"github.com/go-puzzles/emailsender/pkg/dal/model"
	"github.com/go-puzzles/puzzles/cores"
	"github.com/go-puzzles/puzzles/goredis"
	"github.com/go-puzzles/puzzles/pflags"
	"github.com/go-puzzles/puzzles/pgorm"
	"github.com/go-puzzles/puzzles/plog"

	emailSenderGrpc "github.com/go-puzzles/emailsender/api/grpc"
	consulpuzzle "github.com/go-puzzles/puzzles/cores/puzzles/consul-puzzle"
	grpcpuzzle "github.com/go-puzzles/puzzles/cores/puzzles/grpc-puzzle"
	grpcuipuzzle "github.com/go-puzzles/puzzles/cores/puzzles/grpcui-puzzle"
)

var (
	port            = pflags.Int("port", 28080, "port to listen on")
	mysqlConfigFlag = pflags.Struct("mysql", (*pgorm.MysqlConfig)(nil), "mysql config")
	redisConfFlag   = pflags.Struct("redis", (*goredis.RedisConf)(nil), "redis auth config")
	emailConfFlag   = pflags.Struct("email", (*email.Config)(nil), "email smtp config")
)

func main() {
	pflags.Parse()

	mysqlConf := new(pgorm.MysqlConfig)
	plog.PanicError(mysqlConfigFlag(mysqlConf))

	emailConf := new(email.Config)
	plog.PanicError(emailConfFlag(emailConf))

	redisConf := new(goredis.RedisConf)
	plog.PanicError(redisConfFlag(redisConf))

	plog.PanicError(pgorm.RegisterSqlModelWithConf(mysqlConf, model.AllTables()...))
	plog.PanicError(pgorm.AutoMigrate(mysqlConf, pgorm.WithDialIgnoreNotFound()))
	db := pgorm.GetDbByConf(mysqlConf)

	redisClient := redisConf.DialRedisClient()

	emailSendApp := app.NewEmailSenderApp(app.NewAppContext(db, redisClient, emailConf))
	g := emailSenderGrpc.SetupGrpcServer(emailSendApp)

	srv := cores.NewPuzzleCore(
		cores.WithService(pflags.GetServiceName()),
		cores.WithDaemonWorker(emailSendApp.RunEmailAsyncTask),
		consulpuzzle.WithConsulRegister(),
		grpcpuzzle.WithCoreGrpcPuzzle(g),
		grpcuipuzzle.WithCoreGrpcUI(),
	)

	plog.PanicError(cores.Start(srv, port()))
}
