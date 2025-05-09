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

	"github.com/go-puzzles/emailsender/api/grpc/emailsendpb"
	"github.com/go-puzzles/emailsender/app/dto"
)

type EmailHandler interface {
	SendEmail(ctx context.Context, req *dto.SendEmailRequest) error
	SendEmailAsync(ctx context.Context, req *dto.SendEmailRequest) error
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

func (es *EmailSendGrpcServer) Send(ctx context.Context, in *emailsendpb.SendEmailRequest) (*emailsendpb.SendEmailResponse, error) {
	err := es.handler.SendEmail(ctx, &dto.SendEmailRequest{
		Subject: in.GetSubject(),
		From:    in.GetFrom(),
		To:      in.GetTo(),
		Body:    in.GetBody(),
	})

	if err != nil {
		return nil, err
	}

	return &emailsendpb.SendEmailResponse{}, nil
}

func (es *EmailSendGrpcServer) SendAsync(ctx context.Context, in *emailsendpb.SendEmailRequest) (*emailsendpb.SendEmailResponse, error) {
	err := es.handler.SendEmailAsync(ctx, &dto.SendEmailRequest{
		Subject: in.GetSubject(),
		From:    in.GetFrom(),
		To:      in.GetTo(),
		Body:    in.GetBody(),
	})

	if err != nil {
		return nil, err
	}

	return &emailsendpb.SendEmailResponse{}, nil
}
