// File:		grpc.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package emailSenderGrpc

import (
	"github.com/go-puzzles/emailsender/api/grpc/emailsendpb"
	"github.com/go-puzzles/emailsender/api/grpc/server"
	"google.golang.org/grpc"
)

type EmailSendApp interface {
	server.EmailHandler
}

func SetupGrpcServer(a EmailSendApp) func(srv *grpc.Server) {
	return func(srv *grpc.Server) {
		emailSendServer := server.NewEmailSenderGrpcServer(a)
		emailsendpb.RegisterEmailSenderServer(srv, emailSendServer)
	}
}
