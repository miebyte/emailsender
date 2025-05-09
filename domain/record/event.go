// File:		event.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package record

import (
	"context"

	"github.com/go-puzzles/emailsender/domain/event"
)

type EmailStatusEvent struct {
	ID     int
	Status int
}

func (e EmailStatusEvent) EventName() string {
	return "email.sent.status"
}

type Event interface {
	UpdateRecordStatusEvent(ctx context.Context, event event.Event) error
}
