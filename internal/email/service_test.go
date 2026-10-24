package email

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var validMessage = Message{From: "Sender <sender@example.com>", To: "receiver@example.com", Subject: "hello", Body: "body"}

type fakeSender struct {
	send func(context.Context, Message) error
}

func (f fakeSender) Send(ctx context.Context, message Message) error {
	if f.send != nil {
		return f.send(ctx, message)
	}
	return nil
}

type fakeQueue struct {
	enqueue func(context.Context, Task) error
	dequeue func(context.Context) (Task, error)
}

func (f fakeQueue) Enqueue(ctx context.Context, task Task) error {
	if f.enqueue != nil {
		return f.enqueue(ctx, task)
	}
	return nil
}

func (f fakeQueue) Dequeue(ctx context.Context) (Task, error) {
	if f.dequeue != nil {
		return f.dequeue(ctx)
	}
	return Task{}, ErrQueueEmpty
}

type fakeRecords struct {
	create func(context.Context, Message, Status) (int, error)
	update func(context.Context, int, Status) error
}

func (f fakeRecords) Create(ctx context.Context, message Message, status Status) (int, error) {
	if f.create != nil {
		return f.create(ctx, message, status)
	}
	return 1, nil
}

func (f fakeRecords) UpdateStatus(ctx context.Context, id int, status Status) error {
	if f.update != nil {
		return f.update(ctx, id, status)
	}
	return nil
}

func newTestService(t *testing.T, sender Sender, queue TaskQueue, records RecordStore) *Service {
	t.Helper()
	s, err := NewService(sender, queue, records)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func assertResultContext(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := ctx.Err(); err != nil {
		t.Fatalf("result context is canceled: %v", err)
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > recordTimeout || time.Until(deadline) <= 0 {
		t.Fatal("result persistence must have a bounded, live context")
	}
}

func TestNewServiceRequiresDependencies(t *testing.T) {
	for _, dependencies := range []struct {
		sender  Sender
		queue   TaskQueue
		records RecordStore
	}{
		{nil, fakeQueue{}, fakeRecords{}},
		{fakeSender{}, nil, fakeRecords{}},
		{fakeSender{}, fakeQueue{}, nil},
	} {
		if _, err := NewService(dependencies.sender, dependencies.queue, dependencies.records); err == nil {
			t.Fatal("expected missing dependency error")
		}
	}
}

func TestSendPreservesSuccessfulResultAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	message := validMessage
	message.ContentType = "text/html"
	message.Body = "<p>您好</p>"
	var calls []string
	s := newTestService(t, fakeSender{send: func(gotCtx context.Context, got Message) error {
		if gotCtx != ctx || got != message {
			t.Fatal("sender did not receive request context and message")
		}
		calls = append(calls, "send")
		cancel()
		return nil
	}}, fakeQueue{}, fakeRecords{create: func(recordCtx context.Context, got Message, status Status) (int, error) {
		assertResultContext(t, recordCtx)
		if got != message || status != StatusSent {
			t.Fatal("successful SMTP send must create a sent record")
		}
		calls = append(calls, "record")
		return 1, nil
	}})
	if err := s.Send(ctx, message); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"send", "record"}) {
		t.Fatalf("unexpected operation order: %v", calls)
	}
}

func TestSendDoesNotRecordSMTPFailure(t *testing.T) {
	smtpErr := errors.New("SMTP rejected")
	s := newTestService(t, fakeSender{send: func(context.Context, Message) error { return smtpErr }}, fakeQueue{}, fakeRecords{
		create: func(context.Context, Message, Status) (int, error) {
			t.Fatal("failed synchronous send must not create a record")
			return 0, nil
		},
	})
	if err := s.Send(context.Background(), validMessage); !errors.Is(err, smtpErr) {
		t.Fatalf("expected SMTP failure, got %v", err)
	}
}

func TestSendReturnsRecordFailure(t *testing.T) {
	recordErr := errors.New("database unavailable")
	s := newTestService(t, fakeSender{}, fakeQueue{}, fakeRecords{create: func(context.Context, Message, Status) (int, error) {
		return 0, recordErr
	}})
	if err := s.Send(context.Background(), validMessage); !errors.Is(err, recordErr) {
		t.Fatalf("expected persistence failure, got %v", err)
	}
}

func TestSendAsyncCreatesRecordBeforeEnqueue(t *testing.T) {
	ctx := context.Background()
	message := validMessage
	message.ContentType = "text/html"
	message.Body = "<p>您好</p>"
	var calls []string
	s := newTestService(t, fakeSender{}, fakeQueue{enqueue: func(queueCtx context.Context, task Task) error {
		if queueCtx != ctx || task != (Task{ID: 42, Message: message}) {
			t.Fatal("queue did not receive stored task and request context")
		}
		calls = append(calls, "enqueue")
		return nil
	}}, fakeRecords{create: func(recordCtx context.Context, got Message, status Status) (int, error) {
		if recordCtx != ctx || got != message || status != StatusWaiting {
			t.Fatal("expected waiting record and request context")
		}
		calls = append(calls, "create")
		return 42, nil
	}})
	if err := s.SendAsync(ctx, message); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"create", "enqueue"}) {
		t.Fatalf("unexpected operation order: %v", calls)
	}
}

func TestSendAsyncDoesNotEnqueueWhenRecordFails(t *testing.T) {
	recordErr := errors.New("database unavailable")
	s := newTestService(t, fakeSender{}, fakeQueue{enqueue: func(context.Context, Task) error {
		t.Fatal("must not enqueue an unrecorded task")
		return nil
	}}, fakeRecords{create: func(context.Context, Message, Status) (int, error) { return 0, recordErr }})
	if err := s.SendAsync(context.Background(), validMessage); !errors.Is(err, recordErr) {
		t.Fatalf("expected record error, got %v", err)
	}
}

func TestSendAsyncCompensatesEnqueueFailureAfterCancellation(t *testing.T) {
	for _, compensationErr := range []error{nil, errors.New("status update failed")} {
		t.Run(fmt.Sprint("compensation_error=", compensationErr), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queueErr := errors.New("queue unavailable")
			updated := false
			s := newTestService(t, fakeSender{}, fakeQueue{enqueue: func(context.Context, Task) error {
				cancel()
				return queueErr
			}}, fakeRecords{update: func(recordCtx context.Context, id int, status Status) error {
				assertResultContext(t, recordCtx)
				if id != 1 || status != StatusFailed {
					t.Fatal("enqueue failure must mark the waiting record failed")
				}
				updated = true
				return compensationErr
			}})
			err := s.SendAsync(ctx, validMessage)
			if !updated || !errors.Is(err, queueErr) || compensationErr != nil && !errors.Is(err, compensationErr) {
				t.Fatalf("missing compensation or error: updated=%v, error=%v", updated, err)
			}
		})
	}
}

func TestServiceRejectsInvalidOrCanceledInputBeforeIO(t *testing.T) {
	for _, async := range []bool{false, true} {
		for _, invalid := range []bool{false, true} {
			t.Run(fmt.Sprintf("async=%v/invalid=%v", async, invalid), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				message := validMessage
				expectedErr := context.Canceled
				if invalid {
					message.To = "bad address"
					expectedErr = ErrInvalidMessage
				} else {
					cancel()
				}
				s := newTestService(t, fakeSender{send: func(context.Context, Message) error {
					t.Fatal("unexpected SMTP call")
					return nil
				}}, fakeQueue{}, fakeRecords{create: func(context.Context, Message, Status) (int, error) {
					t.Fatal("unexpected database call")
					return 0, nil
				}})
				var err error
				if async {
					err = s.SendAsync(ctx, message)
				} else {
					err = s.Send(ctx, message)
				}
				if !errors.Is(err, expectedErr) {
					t.Fatalf("expected %v, got %v", expectedErr, err)
				}
			})
		}
	}
}

func TestServiceRejectsInvalidContentTypeBeforeIO(t *testing.T) {
	s := newTestService(t, fakeSender{send: func(context.Context, Message) error {
		t.Fatal("invalid content type reached SMTP")
		return nil
	}}, fakeQueue{enqueue: func(context.Context, Task) error {
		t.Fatal("invalid content type reached the queue")
		return nil
	}}, fakeRecords{create: func(context.Context, Message, Status) (int, error) {
		t.Fatal("invalid content type reached storage")
		return 0, nil
	}})
	message := validMessage
	message.ContentType = "application/json"
	for _, async := range []bool{false, true} {
		send := s.Send
		if async {
			send = s.SendAsync
		}
		if err := send(context.Background(), message); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("async=%v: expected invalid message, got %v", async, err)
		}
	}
}

func TestRunContinuesAfterSMTPFailureAndPersistsResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	message := validMessage
	message.ContentType = "text/html"
	message.Body = "<p>您好</p>"
	var dequeued, sent int
	var statuses []Status
	s := newTestService(t, fakeSender{send: func(_ context.Context, got Message) error {
		if got != message {
			t.Fatalf("worker changed the HTML message: %+v", got)
		}
		sent++
		if sent == 1 {
			return errors.New("SMTP temporarily rejected")
		}
		cancel()
		return nil
	}}, fakeQueue{dequeue: func(context.Context) (Task, error) {
		dequeued++
		return Task{ID: dequeued, Message: message}, nil
	}}, fakeRecords{update: func(recordCtx context.Context, id int, status Status) error {
		assertResultContext(t, recordCtx)
		if id != len(statuses)+1 {
			t.Fatalf("unexpected task ID %d", id)
		}
		statuses = append(statuses, status)
		return nil
	}})
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected worker cancellation, got %v", err)
	}
	if sent != 2 || dequeued != 2 || !reflect.DeepEqual(statuses, []Status{StatusFailed, StatusSent}) {
		t.Fatalf("unexpected processing: sent=%d dequeued=%d statuses=%v", sent, dequeued, statuses)
	}
}

func TestRunRejectsInvalidContentType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	message := validMessage
	message.ContentType = "text/html\r\nBcc: other@example.com"
	updated := false
	s := newTestService(t, fakeSender{send: func(context.Context, Message) error {
		t.Fatal("invalid queued message reached SMTP")
		return nil
	}}, fakeQueue{dequeue: func(context.Context) (Task, error) {
		return Task{ID: 42, Message: message}, nil
	}}, fakeRecords{update: func(_ context.Context, id int, status Status) error {
		if id != 42 || status != StatusFailed {
			t.Fatal("invalid queued message must be marked failed")
		}
		updated = true
		cancel()
		return nil
	}})
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) || !updated {
		t.Fatalf("expected failed result then cancellation: updated=%v err=%v", updated, err)
	}
}

func TestRunMarksDequeuedTaskFailedWhenCanceledBeforeSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updated := false
	s := newTestService(t, fakeSender{send: func(context.Context, Message) error {
		t.Fatal("worker must not send another email after cancellation")
		return nil
	}}, fakeQueue{dequeue: func(context.Context) (Task, error) {
		cancel()
		return Task{ID: 42, Message: validMessage}, nil
	}}, fakeRecords{update: func(recordCtx context.Context, id int, status Status) error {
		assertResultContext(t, recordCtx)
		if id != 42 || status != StatusFailed {
			t.Fatal("dequeued task was not marked failed")
		}
		updated = true
		return nil
	}})
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) || !updated {
		t.Fatalf("expected failed result then cancellation: updated=%v err=%v", updated, err)
	}
}

func TestRunPersistsSendCancellationAsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updated := false
	s := newTestService(t, fakeSender{send: func(context.Context, Message) error {
		cancel()
		return ctx.Err()
	}}, fakeQueue{dequeue: func(context.Context) (Task, error) { return Task{ID: 1, Message: validMessage}, nil }},
		fakeRecords{update: func(recordCtx context.Context, _ int, status Status) error {
			assertResultContext(t, recordCtx)
			updated = status == StatusFailed
			return nil
		}})
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) || !updated {
		t.Fatalf("expected failed result for canceled SMTP: updated=%v err=%v", updated, err)
	}
}

func TestRunReturnsQueueAndStatusFailures(t *testing.T) {
	t.Run("queue", func(t *testing.T) {
		queueErr := errors.New("queue unavailable")
		s := newTestService(t, fakeSender{}, fakeQueue{dequeue: func(context.Context) (Task, error) { return Task{}, queueErr }}, fakeRecords{})
		if err := s.Run(context.Background()); !errors.Is(err, queueErr) {
			t.Fatalf("expected queue error, got %v", err)
		}
	})
	t.Run("status", func(t *testing.T) {
		recordErr := errors.New("database unavailable")
		s := newTestService(t, fakeSender{}, fakeQueue{dequeue: func(context.Context) (Task, error) { return Task{ID: 1, Message: validMessage}, nil }},
			fakeRecords{update: func(context.Context, int, Status) error { return recordErr }})
		if err := s.Run(context.Background()); !errors.Is(err, recordErr) {
			t.Fatalf("expected persistence error, got %v", err)
		}
	})
	t.Run("cancel compensation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		recordErr := errors.New("database unavailable")
		s := newTestService(t, fakeSender{}, fakeQueue{dequeue: func(context.Context) (Task, error) {
			cancel()
			return Task{ID: 1, Message: validMessage}, nil
		}}, fakeRecords{update: func(context.Context, int, Status) error { return recordErr }})
		if err := s.Run(ctx); !errors.Is(err, recordErr) || errors.Is(err, context.Canceled) {
			t.Fatalf("persistence failure must not be classified as normal shutdown, got %v", err)
		}
	})
}

func TestRunContinuesAfterEmptyQueueAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	s := newTestService(t, fakeSender{}, fakeQueue{dequeue: func(context.Context) (Task, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		return Task{}, ErrQueueEmpty
	}}, fakeRecords{})
	if err := s.Run(ctx); !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatalf("expected retry then cancellation: calls=%d err=%v", calls, err)
	}
}

func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		valid   bool
	}{
		{"valid", validMessage, true},
		{"empty optional fields", Message{From: validMessage.From, To: validMessage.To}, true},
		{"plain text", Message{From: validMessage.From, To: validMessage.To, ContentType: "text/plain"}, true},
		{"HTML", Message{From: validMessage.From, To: validMessage.To, ContentType: "text/html"}, true},
		{"unsupported content type", Message{From: validMessage.From, To: validMessage.To, ContentType: "application/json"}, false},
		{"content type parameters", Message{From: validMessage.From, To: validMessage.To, ContentType: "text/html; charset=UTF-8"}, false},
		{"content type CRLF", Message{From: validMessage.From, To: validMessage.To, ContentType: "text/html\r\nBcc: other@example.com"}, false},
		{"empty from", Message{To: validMessage.To}, false},
		{"empty to", Message{From: validMessage.From}, false},
		{"invalid address", Message{From: validMessage.From, To: "not an email"}, false},
		{"multiple recipients", Message{From: validMessage.From, To: "a@example.com, b@example.com"}, false},
		{"from CRLF", Message{From: "sender@example.com\r\nBcc: other@example.com", To: validMessage.To}, false},
		{"to LF", Message{From: validMessage.From, To: "receiver@example.com\n"}, false},
		{"subject CR", Message{From: validMessage.From, To: validMessage.To, Subject: "subject\rBcc: other@example.com"}, false},
		{"subject character boundary", Message{From: validMessage.From, To: validMessage.To, Subject: strings.Repeat("邮", 255)}, true},
		{"subject too long", Message{From: validMessage.From, To: validMessage.To, Subject: strings.Repeat("邮", 256)}, false},
		{"address character boundary", Message{From: strings.Repeat("名", 299) + " <sender@example.com>", To: validMessage.To}, true},
		{"address too long", Message{From: strings.Repeat("名", 300) + " <sender@example.com>", To: validMessage.To}, false},
		{"body byte boundary", Message{From: validMessage.From, To: validMessage.To, Body: strings.Repeat("x", 65535)}, true},
		{"body too large", Message{From: validMessage.From, To: validMessage.To, Body: strings.Repeat("邮", 21846)}, false},
		{"body allows line breaks", Message{From: validMessage.From, To: validMessage.To, Body: "line 1\r\nline 2\n"}, true},
		{"invalid UTF8", Message{From: validMessage.From, To: validMessage.To, Body: "\xff"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if tt.valid && err != nil || !tt.valid && !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("valid=%v, got %v", tt.valid, err)
			}
		})
	}
}
