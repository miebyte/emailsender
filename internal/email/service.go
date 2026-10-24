package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/miebyte/goutils/logging"
)

const recordTimeout = 5 * time.Second

type Service struct {
	sender  Sender
	queue   TaskQueue
	records RecordStore
}

func NewService(sender Sender, queue TaskQueue, records RecordStore) (*Service, error) {
	if sender == nil || queue == nil || records == nil {
		return nil, errors.New("email sender, queue and record store are required")
	}
	return &Service{sender: sender, queue: queue, records: records}, nil
}

func (s *Service) Send(ctx context.Context, message Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.sender.Send(ctx, message); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	// SMTP has accepted the email. Preserve that result if the caller disconnects.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if _, err := s.records.Create(recordCtx, message, StatusSent); err != nil {
		return fmt.Errorf("record sent email: %w", err)
	}
	return nil
}

func (s *Service) SendAsync(ctx context.Context, message Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := s.records.Create(ctx, message, StatusWaiting)
	if err != nil {
		return fmt.Errorf("create waiting email: %w", err)
	}
	if err := s.queue.Enqueue(ctx, Task{ID: id, Message: message}); err != nil {
		queueErr := fmt.Errorf("enqueue email %d: %w", id, err)
		if updateErr := s.updateStatus(ctx, id, StatusFailed); updateErr != nil {
			return errors.Join(queueErr, updateErr)
		}
		return queueErr
	}
	return nil
}

func (s *Service) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		task, err := s.queue.Dequeue(ctx)
		if errors.Is(err, ErrQueueEmpty) {
			continue
		}
		if err != nil {
			return fmt.Errorf("dequeue email: %w", err)
		}
		if err := ctx.Err(); err != nil {
			if updateErr := s.updateStatus(ctx, task.ID, StatusFailed); updateErr != nil {
				return updateErr
			}
			return err
		}

		sendErr := task.Message.Validate()
		if sendErr == nil {
			sendErr = s.sender.Send(ctx, task.Message)
		}
		status := StatusSent
		if sendErr != nil {
			status = StatusFailed
			logging.Errorc(ctx, "send queued email %d: %v", task.ID, sendErr)
		}
		if err := s.updateStatus(ctx, task.ID, status); err != nil {
			return err
		}
	}
}

func (s *Service) updateStatus(ctx context.Context, id int, status Status) error {
	// A dequeued task must receive its final status even during graceful shutdown.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	if err := s.records.UpdateStatus(recordCtx, id, status); err != nil {
		return fmt.Errorf("update email %d status: %w", id, err)
	}
	return nil
}
