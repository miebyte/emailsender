package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/miebyte/emailsender/api/grpc/emailsendpb"
	"github.com/miebyte/emailsender/internal/email"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeHandler struct {
	send func(context.Context, email.Message) error
}

func (f fakeHandler) Send(ctx context.Context, message email.Message) error {
	return f.send(ctx, message)
}

func (f fakeHandler) SendAsync(ctx context.Context, message email.Message) error {
	return f.send(ctx, message)
}

func TestEmailRPCForwardsMessageAndContext(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprint("async=", async), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := email.Message{From: "sender@example.com", To: "receiver@example.com", Subject: "subject", Body: "body"}
			called := false
			srv := NewEmailSenderGrpcServer(fakeHandler{send: func(gotCtx context.Context, got email.Message) error {
				if gotCtx != ctx || got != expected {
					t.Fatalf("unexpected application call: %v", got)
				}
				called = true
				return nil
			}})
			rpc := srv.Send
			if async {
				rpc = srv.SendAsync
			}
			response, err := rpc(ctx, &emailsendpb.SendEmailRequest{
				From: expected.From, To: expected.To, Subject: expected.Subject, Body: expected.Body,
			})
			if err != nil || response == nil || !called {
				t.Fatalf("expected successful empty response: response=%v called=%v err=%v", response, called, err)
			}
		})
	}
}

func TestEmailRPCMapsErrorsWithoutLeakingInfrastructureDetails(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	deadlineCtx, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		code codes.Code
	}{
		{"invalid", context.Background(), fmt.Errorf("%w: invalid recipient", email.ErrInvalidMessage), codes.InvalidArgument},
		{"canceled", canceledCtx, fmt.Errorf("send email: %w", context.Canceled), codes.Canceled},
		{"deadline", deadlineCtx, fmt.Errorf("send email: %w", context.DeadlineExceeded), codes.DeadlineExceeded},
		{"internal cancellation", context.Background(), fmt.Errorf("record email: %w", context.Canceled), codes.Internal},
		{"internal deadline", context.Background(), fmt.Errorf("record email: %w", context.DeadlineExceeded), codes.Internal},
		{"cancellation with compensation error", canceledCtx, errors.Join(context.Canceled, errors.New("database dial mysql.private.example failed")), codes.Canceled},
		{"database", context.Background(), errors.New("database dial mysql.private.example:3306 failed"), codes.Internal},
		{"SMTP", context.Background(), errors.New("SMTP server smtp.private.example rejected"), codes.Internal},
	}
	for _, async := range []bool{false, true} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("async=%v/%s", async, tt.name), func(t *testing.T) {
				srv := NewEmailSenderGrpcServer(fakeHandler{send: func(context.Context, email.Message) error { return tt.err }})
				rpc := srv.Send
				if async {
					rpc = srv.SendAsync
				}
				response, err := rpc(tt.ctx, &emailsendpb.SendEmailRequest{})
				if response != nil || status.Code(err) != tt.code {
					t.Fatalf("expected %v and no response, got response=%v err=%v", tt.code, response, err)
				}
				if strings.Contains(err.Error(), ".private.example") {
					t.Fatalf("infrastructure details leaked: %v", err)
				}
			})
		}
	}
}
