// File:		record.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package recordSrv

import (
	"context"

	"github.com/pkg/errors"

	"github.com/miebyte/emailsender/domain/record"
)

var _ record.Service = (*recordService)(nil)

type recordService struct {
	recordRepo record.Repository
}

func NewRecordService(recordRepo record.Repository) *recordService {
	return &recordService{
		recordRepo: recordRepo,
	}
}

func (rs *recordService) CreateRecord(ctx context.Context, record *record.EmailSendRecord) (*record.EmailSendRecord, error) {
	if record == nil {
		return nil, errors.New("record cannot be nil")
	}

	err := rs.recordRepo.SaveEmail(ctx, record)
	if err != nil {
		return nil, errors.Wrap(err, "failed to save email record")
	}

	savedRecord, err := rs.recordRepo.GetById(ctx, record.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get saved email record")
	}

	return savedRecord, nil
}

func (rs *recordService) UpdateRecord(ctx context.Context, record *record.EmailSendRecord) error {
	if record == nil {
		return errors.New("record cannot be nil")
	}

	err := rs.recordRepo.UpdateStatus(ctx, record.ID, record.Status)
	if err != nil {
		return errors.Wrap(err, "failed to update email record status")
	}

	return nil
}

func (rs *recordService) GetRecordsByStatus(ctx context.Context, status record.SendStatus) ([]*record.EmailSendRecord, error) {
	records, err := rs.recordRepo.GetByStatus(ctx, status)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get email records by status")
	}

	return records, nil
}
