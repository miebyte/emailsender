// File:		main.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"gitea.hoven.com/core/emailsender/app"
	"gitea.hoven.com/core/emailsender/pkg/email"
	"github.com/go-puzzles/puzzles/cores"
	"github.com/go-puzzles/puzzles/goredis"
	"github.com/go-puzzles/puzzles/pflags"
	"github.com/go-puzzles/puzzles/plog"

	emailSenderGrpc "gitea.hoven.com/core/emailsender/api/grpc"
	consulpuzzle "github.com/go-puzzles/puzzles/cores/puzzles/consul-puzzle"
	grpcpuzzle "github.com/go-puzzles/puzzles/cores/puzzles/grpc-puzzle"
	grpcuipuzzle "github.com/go-puzzles/puzzles/cores/puzzles/grpcui-puzzle"
)

var (
	port          = pflags.Int("port", 28080, "port to listen on")
	redisConfFlag = pflags.Struct("redis", (*goredis.RedisConf)(nil), "redis auth config")
	emailConfFlag = pflags.Struct("email", (*email.Config)(nil), "email smtp config")
)

func main() {
	pflags.Parse()

	emailConf := new(email.Config)
	plog.PanicError(emailConfFlag(emailConf))

	redisConf := new(goredis.RedisConf)
	plog.PanicError(redisConfFlag(redisConf))

	redisClient := redisConf.DialRedisClient()
	emailSender := email.NewNetEasySender(emailConf, redisClient)

	emailSendApp := app.NewEmailSenderApp(app.NewAppContext(emailSender, redisClient))
	g := emailSenderGrpc.SetupGrpcServer(emailSendApp)

	srv := cores.NewPuzzleCore(
		cores.WithService(pflags.GetServiceName()),
		consulpuzzle.WithConsulRegister(),
		grpcpuzzle.WithCoreGrpcPuzzle(g),
		grpcuipuzzle.WithCoreGrpcUI(),
	)

	plog.PanicError(cores.Start(srv, port()))
}
