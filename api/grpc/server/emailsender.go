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
	"errors"

	"github.com/miebyte/emailsender/api/grpc/emailsendpb"
	"github.com/miebyte/emailsender/internal/email"
	"github.com/miebyte/goutils/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EmailHandler interface {
	Send(context.Context, email.Message) error
	SendAsync(context.Context, email.Message) error
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
	err := es.handler.Send(ctx, email.Message{
		Subject:     in.GetSubject(),
		From:        in.GetFrom(),
		To:          in.GetTo(),
		Body:        in.GetBody(),
		ContentType: in.GetContentType(),
	})

	if err != nil {
		return nil, rpcError(ctx, err)
	}

	return &emailsendpb.SendEmailResponse{}, nil
}

func (es *EmailSendGrpcServer) SendAsync(ctx context.Context, in *emailsendpb.SendEmailRequest) (*emailsendpb.SendEmailResponse, error) {
	err := es.handler.SendAsync(ctx, email.Message{
		Subject:     in.GetSubject(),
		From:        in.GetFrom(),
		To:          in.GetTo(),
		Body:        in.GetBody(),
		ContentType: in.GetContentType(),
	})

	if err != nil {
		return nil, rpcError(ctx, err)
	}

	return &emailsendpb.SendEmailResponse{}, nil
}

func rpcError(ctx context.Context, err error) error {
	if !errors.Is(err, email.ErrInvalidMessage) {
		logging.Errorc(ctx, "email request failed: %v", err)
	}
	switch {
	case errors.Is(err, email.ErrInvalidMessage):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded) && errors.Is(ctx.Err(), context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "email request failed")
	}
}
