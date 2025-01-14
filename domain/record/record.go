// File:		record.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package record

import "time"

type EmailSendRecord struct {
	ID      int
	From    string
	To      string
	Subject string
	Message string
	Status  SendStatus
	SendAt  time.Time
}

func NewEmailRecord(from, to, subject, message string, status SendStatus) *EmailSendRecord {
	r := &EmailSendRecord{
		From:    from,
		To:      to,
		Subject: subject,
		Message: message,
		Status:  status,
		SendAt:  time.Now(),
	}

	if status == StatusSent {
		r.SendAt = time.Now()
	}

	return r
}

type SendStatus int

const (
	StatusFailed SendStatus = iota
	StatusSent
	StatusWaiting
)
