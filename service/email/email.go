// File:		email.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package emailSrv

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"time"

	"gitea.hoven.com/core/emailsender/domain/email"
	"gitea.hoven.com/core/emailsender/domain/event"
	"github.com/go-puzzles/puzzles/goredis"
	"github.com/go-puzzles/puzzles/plog"
	"github.com/go-puzzles/puzzles/pqueue"
	"github.com/pkg/errors"
)

var _ email.Service = (*emailService)(nil)

type emailService struct {
	conf       *email.Config
	auth       smtp.Auth
	tlsConf    *tls.Config
	eventBus   event.EventBus
	asyncQueue *pqueue.RedisQueue[*email.AsyncEmailTask]
}

func NewEmailService(
	conf *email.Config,
	eventBus event.EventBus,
	redisClient *goredis.PuzzleRedisClient,
) *emailService {
	auth := smtp.PlainAuth("", conf.Identifier, conf.Credential, conf.SmtpServer)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         conf.SmtpServer,
	}

	asyncQueue := pqueue.NewRedisQueueWithClient[*email.AsyncEmailTask](redisClient, conf.AsyncQueue)
	return &emailService{
		conf:       conf,
		auth:       auth,
		tlsConf:    tlsConfig,
		eventBus:   eventBus,
		asyncQueue: asyncQueue,
	}
}

func (e *emailService) LoopAsyncTask(ctx context.Context, callbackFn func(int, error)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		task, err := e.asyncQueue.Dequeue()
		if errors.Is(err, pqueue.QueueEmptyError) {
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			plog.Errorc(ctx, "async task dequeue error: %v", err)
			return errors.Wrap(err, "asyncTaskDequeue")
		}

		err = e.SendMsg(ctx, task.Subject, task.FromEmail, task.TargetEmail, []byte(task.Msg))
		if err != nil {
			plog.Errorc(ctx, "send msg error: %v", err)
		}

		callbackFn(task.Id, err)
	}
}

func (e *emailService) getClient(target string) (*smtp.Client, error) {
	conn, err := tls.Dial("tcp", e.conf.SmtpUrl(), e.tlsConf)
	if err != nil {
		return nil, errors.Wrap(err, "dialSmtp")
	}

	client, err := smtp.NewClient(conn, e.conf.SmtpServer)
	if err != nil {
		return nil, errors.Wrap(err, "newSmtpClient")
	}

	if err = client.Auth(e.auth); err != nil {
		return nil, errors.Wrap(err, "smtpAuth")
	}

	if err = client.Mail(e.conf.Identifier); err != nil {
		return nil, errors.Wrap(err, "smtpMail")
	}
	if err = client.Rcpt(target); err != nil {
		return nil, errors.Wrap(err, "smtpRcpt")
	}

	return client, nil
}

func (e *emailService) wrapMsg(subject, from, target string, msg []byte) []byte {
	subject = fmt.Sprintf("Subject: %s\r\n,", subject)
	fromHeader := fmt.Sprintf("From: %s\r\n", from)
	toHeader := fmt.Sprintf("To: %s\r\n", target)
	contentTypeHeader := "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
	mimeVersionHeader := "MIME-Version: 1.0\r\n"

	data := []byte(fromHeader + toHeader + subject + mimeVersionHeader + contentTypeHeader + "\r\n")
	return append(data, msg...)
}

func (e *emailService) SendMsg(ctx context.Context, subject, from, target string, msg []byte) error {
	client, err := e.getClient(target)
	if err != nil {
		return errors.Wrapf(err, "getSmtpClient: %v", target)
	}
	defer client.Quit()

	w, err := client.Data()
	if err != nil {
		return errors.Wrapf(err, "getWriter: %v", target)
	}
	defer w.Close()

	_, err = w.Write(e.wrapMsg(subject, from, target, msg))
	if err != nil {
		return errors.Wrapf(err, "writeData: %v", target)
	}

	return nil
}

func (e *emailService) AsyncSendMsg(ctx context.Context, id int, subject, from, target string, msg []byte) error {
	task := &email.AsyncEmailTask{
		Id:          id,
		Subject:     subject,
		FromEmail:   from,
		TargetEmail: target,
		Msg:         string(msg),
	}
	return e.asyncQueue.Enqueue(task)
}
