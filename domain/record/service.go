// File:		service.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package record

import "context"

type Service interface {
	CreateRecord(ctx context.Context, record *EmailSendRecord) (*EmailSendRecord, error)
	UpdateRecord(ctx context.Context, record *EmailSendRecord) error
	GetRecordsByStatus(ctx context.Context, status SendStatus) ([]*EmailSendRecord, error)
}
