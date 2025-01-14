// File:		repo.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package record

import "context"

type Repository interface {
	SaveEmail(ctx context.Context, email *EmailSendRecord) error
	UpdateStatus(ctx context.Context, id int, status SendStatus) error
	GetByStatus(ctx context.Context, status SendStatus) ([]*EmailSendRecord, error)
	GetById(ctx context.Context, id int) (*EmailSendRecord, error)
	Delete(ctx context.Context, id int) error
}
