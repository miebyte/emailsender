package email

import (
	"context"
	"errors"
)

type Status int

const (
	StatusFailed Status = iota
	StatusSent
	StatusWaiting
)

var (
	ErrInvalidMessage = errors.New("invalid email message")
	ErrQueueEmpty     = errors.New("email queue is empty")
)

type Message struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	ContentType string `json:"content_type,omitempty"`
}

type Task struct {
	ID      int     `json:"id"`
	Message Message `json:"message"`
}

type Sender interface {
	Send(context.Context, Message) error
}

type TaskQueue interface {
	Enqueue(context.Context, Task) error
	Dequeue(context.Context) (Task, error)
}

type RecordStore interface {
	Create(context.Context, Message, Status) (int, error)
	UpdateStatus(context.Context, int, Status) error
}
