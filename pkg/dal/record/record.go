// File:		record.go
// Created by:	Hoven
// Created on:	2025-01-14
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package recordDal

import (
	"context"
	"time"

	"github.com/miebyte/emailsender/domain/record"
	"github.com/miebyte/emailsender/pkg/dal/base"
	"github.com/miebyte/emailsender/pkg/dal/model"
	"github.com/pkg/errors"
)

var _ record.Repository = (*recordRepo)(nil)

type recordRepo struct {
	db *base.BaseDal
}

func NewRecordRepo(db *base.BaseDal) record.Repository {
	return &recordRepo{
		db: db,
	}
}

func (rp *recordRepo) SaveEmail(ctx context.Context, email *record.EmailSendRecord) error {
	r := &model.Record{
		From:      email.From,
		To:        email.To,
		Subject:   email.Subject,
		Message:   email.Message,
		Status:    email.Status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if email.Status == record.StatusSent {
		now := time.Now()
		r.SentAt = &now
	}

	err := rp.db.Db(ctx).Record.Create(r)
	if err != nil {
		return errors.Wrap(err, "create record failed")
	}

	email.ID = r.ID
	return nil
}

func (rp *recordRepo) UpdateStatus(ctx context.Context, id int, status record.SendStatus) error {
	updates := &model.Record{
		ID:     id,
		Status: status,
	}

	if status == record.StatusSent {
		now := time.Now()
		updates.SentAt = &now
	}

	_, err := rp.db.Db(ctx).Record.Where(rp.db.Record.ID.Eq(id)).Updates(updates)
	if err != nil {
		return errors.Wrap(err, "update record status failed")
	}
	return nil
}

func (rp *recordRepo) GetByStatus(ctx context.Context, status record.SendStatus) ([]*record.EmailSendRecord, error) {
	records, err := rp.db.Db(ctx).Record.Where(rp.db.Record.Status.Eq(int(status))).Find()
	if err != nil {
		return nil, errors.Wrap(err, "query records failed")
	}

	result := make([]*record.EmailSendRecord, 0, len(records))
	for _, r := range records {
		sendAt := time.Time{}
		if r.SentAt != nil {
			sendAt = *r.SentAt
		}

		result = append(result, &record.EmailSendRecord{
			ID:      r.ID,
			From:    r.From,
			To:      r.To,
			Subject: r.Subject,
			Message: r.Message,
			Status:  r.Status,
			SendAt:  sendAt,
		})
	}

	return result, nil
}

func (rp *recordRepo) GetById(ctx context.Context, id int) (*record.EmailSendRecord, error) {
	r, err := rp.db.Db(ctx).Record.Where(rp.db.Record.ID.Eq(id)).First()
	if err != nil {
		return nil, errors.Wrap(err, "query record failed")
	}

	sendAt := time.Time{}
	if r.SentAt != nil {
		sendAt = *r.SentAt
	}

	return &record.EmailSendRecord{
		ID:      r.ID,
		From:    r.From,
		To:      r.To,
		Subject: r.Subject,
		Message: r.Message,
		Status:  r.Status,
		SendAt:  sendAt,
	}, nil
}

func (rp *recordRepo) Delete(ctx context.Context, id int) error {
	_, err := rp.db.Db(ctx).Record.Where(rp.db.Record.ID.Eq(id)).Delete()
	if err != nil {
		return errors.Wrap(err, "delete record failed")
	}
	return nil
}
