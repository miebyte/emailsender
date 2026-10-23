package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/miebyte/emailsender/internal/email"
	"github.com/miebyte/emailsender/internal/storage/models"
	"github.com/miebyte/emailsender/internal/storage/query"
	"gorm.io/gorm"
)

var _ email.RecordStore = (*RecordStore)(nil)

type RecordStore struct {
	q *query.Query
}

func NewRecordStore(db *gorm.DB) (*RecordStore, error) {
	return &RecordStore{q: query.Use(db)}, nil
}

func (s *RecordStore) Create(ctx context.Context, message email.Message, status email.Status) (int, error) {
	now := time.Now()
	record := &models.Record{
		From:      message.From,
		To:        message.To,
		Subject:   message.Subject,
		Body:      message.Body,
		Status:    int(status),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if status == email.StatusSent {
		record.SentAt = &now
	}
	if err := s.q.Record.WithContext(ctx).Create(record); err != nil {
		return 0, fmt.Errorf("create email record: %w", err)
	}
	return record.ID, nil
}

func (s *RecordStore) UpdateStatus(ctx context.Context, id int, status email.Status) error {
	r := s.q.Record
	now := time.Now()
	sentAt := r.SentAt.Null()
	if status == email.StatusSent {
		sentAt = r.SentAt.Value(now)
	}
	// Disable Gen's automatic timestamp assignment because updated_at is explicit.
	result, err := r.WithContext(ctx).Session(&gorm.Session{SkipHooks: true}).Where(r.ID.Eq(id)).UpdateColumnSimple(r.Status.Value(int(status)), r.UpdatedAt.Value(now), sentAt)
	if err != nil {
		return fmt.Errorf("update email record status: %w", err)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("email record %d was not updated", id)
	}
	return nil
}
