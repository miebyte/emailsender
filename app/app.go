// File:		app.go
// Created by:	Hoven
// Created on:	2025-01-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package app

import (
	"context"

	"gitea.hoven.com/core/emailsender/app/dto"
)

type EmailSendApp struct {
	Atx *AppContext
}

func NewEmailSenderApp(atx *AppContext) *EmailSendApp {
	return &EmailSendApp{
		Atx: atx,
	}
}

func (es *EmailSendApp) SendEmail(ctx context.Context, req *dto.SendEmailRequest) error {
	return es.Atx.EmailSender.SendMsg(ctx, req.Subject, req.From, req.To, []byte(req.Body))
}
