// File:		email.go
// Created by:	Hoven
// Created on:	2024-11-19
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"time"

	"github.com/go-puzzles/puzzles/goredis"
	"github.com/go-puzzles/puzzles/plog"
	"github.com/go-puzzles/puzzles/pqueue"
	"github.com/pkg/errors"
)

type AsyncEmailTask struct {
	Subject     string
	FromEmail   string
	TargetEmail string
	Msg         string
}

func (t *AsyncEmailTask) Key() string {
	return "AsyncEmailTask" + t.TargetEmail
}

type EmailSender interface {
	SendMsg(ctx context.Context, subject, from, target string, msg []byte) error
	AsyncSendMsg(ctx context.Context, task *AsyncEmailTask) error
}

type emailSender struct {
	conf    *Config
	auth    smtp.Auth
	tlsConf *tls.Config
	queue   *pqueue.RedisQueue[*AsyncEmailTask]
}

func NewNetEasySender(conf *Config, redisClient *goredis.PuzzleRedisClient) *emailSender {
	auth := smtp.PlainAuth("", conf.Identifier, conf.Credential, conf.SmtpServer)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         conf.SmtpServer,
	}

	queue := pqueue.NewRedisQueueWithClient[*AsyncEmailTask](redisClient, conf.AsyncQueue)
	return &emailSender{
		conf:    conf,
		auth:    auth,
		tlsConf: tlsConfig,
		queue:   queue,
	}
}

func (e *emailSender) LoopAsyncTask(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		task, err := e.queue.Dequeue()
		if errors.Is(err, pqueue.QueueEmptyError) {
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			plog.Errorc(ctx, "async task dequeue error: %v", err)
			return errors.Wrap(err, "asyncTaskDequeue")
		}

		err = e.SendMsg(ctx, task.Subject, task.FromEmail, task.TargetEmail, []byte(task.Msg))
		if err != nil {
			plog.Errorc(ctx, "send email to %s error: %v", task.TargetEmail, err)
			time.Sleep(time.Millisecond * 500)
			continue
		}
	}
}

func (e *emailSender) getClient(target string) (*smtp.Client, error) {
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

func (e *emailSender) wrapMsg(subject, from, target string, msg []byte) []byte {
	subject = fmt.Sprintf("Subject: %s\r\n,", subject)
	fromHeader := fmt.Sprintf("From: %s\r\n", from)
	toHeader := fmt.Sprintf("To: %s\r\n", target)
	contentTypeHeader := "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
	mimeVersionHeader := "MIME-Version: 1.0\r\n"

	data := []byte(fromHeader + toHeader + subject + mimeVersionHeader + contentTypeHeader + "\r\n")
	return append(data, msg...)
}

func (e *emailSender) SendMsg(ctx context.Context, subject, from, target string, msg []byte) error {
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

func (e *emailSender) AsyncSendMsg(ctx context.Context, task *AsyncEmailTask) error {
	return e.queue.Enqueue(task)
}
