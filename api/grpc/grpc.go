// File:		grpc.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package emailSenderGrpc

import (
	"github.com/miebyte/emailsender/api/grpc/emailsendpb"
	"github.com/miebyte/emailsender/api/grpc/server"
	"google.golang.org/grpc"
)

func SetupGrpcServer(srv *grpc.Server, a server.EmailHandler) {
	emailSendServer := server.NewEmailSenderGrpcServer(a)
	emailsendpb.RegisterEmailSenderServer(srv, emailSendServer)
}
