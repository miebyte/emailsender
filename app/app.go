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

	"github.com/miebyte/emailsender/app/dto"
	"github.com/miebyte/emailsender/domain/record"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
)

type EmailSendApp struct {
	Atx *AppContext
}

func NewEmailSenderApp(atx *AppContext) *EmailSendApp {
	return &EmailSendApp{
		Atx: atx,
	}
}

func (es *EmailSendApp) RunEmailAsyncTask(ctx context.Context) error {
	eventPublishFn := func(i int, err error) {
		status := record.StatusSent
		if err != nil {
			status = record.StatusFailed
		}

		err = es.Atx.EventBus.Publish(ctx, &record.EmailStatusEvent{ID: i, Status: int(status)})
		if err != nil {
			logging.Errorc(ctx, "publish email status event failed: %v", err)
			return
		}
	}

	return es.Atx.EmailSrv.LoopAsyncTask(ctx, eventPublishFn)
}

func (es *EmailSendApp) SendEmail(ctx context.Context, req *dto.SendEmailRequest) error {
	err := es.Atx.EmailSrv.SendMsg(ctx, req.Subject, req.From, req.To, []byte(req.Body))
	if err != nil {
		logging.Errorc(ctx, "send email failed: %v", err)
		return err
	}

	r := record.NewEmailRecord(req.From, req.To, req.Subject, req.Body, record.StatusSent)

	_, err = es.Atx.RecordSrv.CreateRecord(ctx, r)
	if err != nil {
		logging.Errorc(ctx, "create email record failed: %v", err)
		return errors.Wrap(err, "createSentRecord")
	}

	return nil
}

func (es *EmailSendApp) SendEmailAsync(ctx context.Context, req *dto.SendEmailRequest) (err error) {
	r := record.NewEmailRecord(req.From, req.To, req.Subject, req.Body, record.StatusWaiting)

	r, err = es.Atx.RecordSrv.CreateRecord(ctx, r)
	if err != nil {
		logging.Errorc(ctx, "create email record failed: %v", err)
		return errors.Wrap(err, "createAsyncRecord")
	}

	err = es.Atx.EmailSrv.AsyncSendMsg(ctx, r.ID, req.Subject, req.From, req.To, []byte(req.Body))
	if err != nil {
		logging.Errorc(ctx, "send email async failed: %v", err)
		return errors.Wrap(err, "sendAsyncEmail")
	}

	return nil
}
