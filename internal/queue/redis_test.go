package queue

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miebyte/emailsender/internal/email"
	"github.com/miebyte/goutils/redisutils"
	"github.com/redis/go-redis/v9"
)

type commandHook struct {
	process func(context.Context, redis.Cmder) error
}

func (h commandHook) DialHook(next redis.DialHook) redis.DialHook     { return next }
func (h commandHook) ProcessHook(redis.ProcessHook) redis.ProcessHook { return h.process }
func (h commandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func testQueue(t *testing.T, process func(context.Context, redis.Cmder) error) *RedisQueue {
	t.Helper()
	client := redis.NewClient(&redis.Options{})
	t.Cleanup(func() { _ = client.Close() })
	client.AddHook(commandHook{process: process})
	q, err := NewRedisQueue(&redisutils.RedisClient{Client: client}, "emails", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestQueueTaskRoundTrip(t *testing.T) {
	task := email.Task{ID: 7, Message: email.Message{
		From: "sender@example.com", To: "recipient@example.com", Subject: "hello", Body: "body",
	}}
	type contextKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), contextKey{}, "request"), 5*time.Second)
	defer cancel()
	var queued []byte
	q := testQueue(t, func(gotCtx context.Context, cmd redis.Cmder) error {
		if gotCtx.Value(contextKey{}) != "request" {
			t.Fatal("request context values were lost")
		}
		if deadline, ok := gotCtx.Deadline(); !ok || time.Until(deadline) <= 0 || time.Until(deadline) > time.Second {
			t.Fatal("queue operation did not receive a bounded deadline")
		}
		switch cmd.Name() {
		case "rpush":
			queued = cmd.Args()[2].([]byte)
			cmd.(*redis.IntCmd).SetVal(1)
		case "blpop":
			cmd.(*redis.StringSliceCmd).SetVal([]string{"emails", string(queued)})
		default:
			t.Fatalf("unexpected command %s", cmd.Name())
		}
		return nil
	})
	if err := q.Enqueue(ctx, task); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(queued, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"id": float64(7), "message": map[string]any{"subject": "hello", "from": "sender@example.com", "to": "recipient@example.com", "body": "body"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected task payload: %s", queued)
	}
	decoded, err := q.Dequeue(ctx)
	if err != nil || decoded != task {
		t.Fatalf("dequeue = %+v, %v", decoded, err)
	}
}

func TestDequeueEmptyAndMalformed(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []string
		err  error
	}{
		{name: "empty", err: redis.Nil},
		{name: "invalid response", data: []string{"emails"}},
		{name: "invalid JSON", data: []string{"emails", "{"}},
		{name: "missing ID", data: []string{"emails", `{}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := testQueue(t, func(_ context.Context, cmd redis.Cmder) error {
				cmd.(*redis.StringSliceCmd).SetVal(tc.data)
				return tc.err
			})
			_, err := q.Dequeue(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.name == "empty" && !errors.Is(err, email.ErrQueueEmpty) {
				t.Fatalf("empty error = %v", err)
			}
		})
	}
}

func TestCanceledQueueOperationsDoNotReachRedis(t *testing.T) {
	q := testQueue(t, func(context.Context, redis.Cmder) error {
		t.Fatal("canceled operation reached Redis")
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Enqueue(ctx, email.Task{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("enqueue = %v", err)
	}
	if _, err := q.Dequeue(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("dequeue = %v", err)
	}
}

func TestStalledRedisIOHasBoundedDeadline(t *testing.T) {
	for _, tc := range []struct {
		name       string
		enqueue    bool
		deadline   bool
		cancelOnce bool
	}{
		{name: "enqueue timeout", enqueue: true},
		{name: "dequeue timeout"},
		{name: "request deadline", deadline: true},
		{name: "request cancellation", cancelOnce: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address, commandSeen := stalledRedis(t)
			client, err := redisutils.NewRedisClient(&redisutils.RedisConfig{
				Server: address, Timeout: 2, ContextTimeoutEnabled: true, MaxRetries: -1,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.Close() })
			q, err := NewRedisQueue(client, "emails", 200*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if !q.client.Options().ContextTimeoutEnabled || q.client.Options().MaxRetries != 0 {
				t.Fatal("Redis client must enable context deadlines and disable retries")
			}
			ctx, cancel := context.WithCancel(context.Background())
			if tc.deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
			}
			defer cancel()
			result := make(chan error, 1)
			go func() {
				if tc.enqueue {
					result <- q.Enqueue(ctx, email.Task{ID: 1})
				} else {
					_, err := q.Dequeue(ctx)
					result <- err
				}
			}()
			select {
			case <-commandSeen:
			case <-time.After(time.Second):
				t.Fatal("Redis command did not reach the socket")
			}
			want := context.DeadlineExceeded
			if tc.cancelOnce {
				cancel()
				want = context.Canceled
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("expected %v, got %v", want, err)
				}
			case <-time.After(time.Second):
				t.Fatal("stalled Redis I/O exceeded its deadline")
			}
		})
	}
}

// Complete Redis' connection handshake, then leave the queue command unanswered.
func stalledRedis(t *testing.T) (string, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	commandSeen := make(chan struct{})
	accepted := make(chan net.Conn, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		accepted <- conn
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			command, err := readRESPCommand(reader)
			if err != nil {
				return
			}
			var reply string
			switch strings.ToLower(command[0]) {
			case "hello":
				reply = "-ERR unknown command 'hello'\r\n"
			case "ping":
				reply = "+PONG\r\n"
			case "rpush", "blpop":
				close(commandSeen)
				_, _ = io.Copy(io.Discard, conn)
				return
			default:
				reply = "+OK\r\n"
			}
			if _, err := io.WriteString(conn, reply); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case conn := <-accepted:
			_ = conn.Close()
		default:
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("fake Redis server did not stop")
		}
	})
	return listener.Addr().String(), commandSeen
}

func readRESPCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	var count int
	if _, err := fmt.Sscanf(line, "*%d\r\n", &count); err != nil || count < 1 {
		return nil, fmt.Errorf("invalid RESP array")
	}
	command := make([]string, count)
	for i := range command {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(line, "$"), "\r\n"))
		if err != nil || length < 0 {
			return nil, fmt.Errorf("invalid RESP bulk string")
		}
		data := make([]byte, length+2)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		command[i] = string(data[:length])
	}
	return command, nil
}
