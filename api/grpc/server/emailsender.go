// File:		emailsender.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package server

import (
	"context"

	"gitea.hoven.com/core/emailsender/api/grpc/emailsendpb"
	"gitea.hoven.com/core/emailsender/app/dto"
)

type EmailHandler interface {
	SendEmail(ctx context.Context, req *dto.SendEmailRequest) error
}

type EmailSendGrpcServer struct {
	emailsendpb.UnimplementedEmailSenderServer
	handler EmailHandler
}

func NewEmailSenderGrpcServer(handler EmailHandler) *EmailSendGrpcServer {
	return &EmailSendGrpcServer{
		handler: handler,
	}
}

func (es *EmailSendGrpcServer) SendEmail(ctx context.Context, in *emailsendpb.SendEmailRequest) (*emailsendpb.SendEmailResponse, error) {
	err := es.handler.SendEmail(ctx, &dto.SendEmailRequest{
		Subject: in.GetSubject(),
		To:      in.GetEmail(),
		Body:    in.GetBody(),
	})

	if err != nil {
		return nil, err
	}

	return &emailsendpb.SendEmailResponse{}, nil
}
