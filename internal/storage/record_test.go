package storage

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/miebyte/emailsender/internal/email"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type nonzeroTime struct{}

func (nonzeroTime) Match(value driver.Value) bool {
	timestamp, ok := value.(time.Time)
	return ok && !timestamp.IsZero()
}

func newTestStore(t *testing.T) (*RecordStore, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = sqlDB.Close()
	})
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewRecordStore(db)
	if err != nil {
		t.Fatal(err)
	}
	return store, mock
}

func TestNewRecordStoreRequiresDatabase(t *testing.T) {
	if _, err := NewRecordStore(nil); err == nil {
		t.Fatal("expected an error for a missing database")
	}
}

func TestCreateRecordsSentTimeOnlyForSentEmail(t *testing.T) {
	message := email.Message{From: "from@example.com", To: "to@example.com", Subject: "subject", Body: "body"}
	for _, status := range []email.Status{email.StatusFailed, email.StatusSent, email.StatusWaiting} {
		store, mock := newTestStore(t)
		var sentAt driver.Value
		if status == email.StatusSent {
			sentAt = nonzeroTime{}
		}
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `records` (`from`,`to`,`subject`,`body`,`content_type`,`status`,`sent_at`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?,?,?,?)")).
			WithArgs(message.From, message.To, message.Subject, message.Body, "text/plain", int(status), sentAt, nonzeroTime{}, nonzeroTime{}).
			WillReturnResult(sqlmock.NewResult(42, 1))
		mock.ExpectCommit()
		id, err := store.Create(context.Background(), message, status)
		if err != nil {
			t.Fatal(err)
		}
		if id != 42 {
			t.Fatalf("create returned ID %d, want 42", id)
		}
	}
}

func TestCreatePersistsContentType(t *testing.T) {
	for _, contentType := range []string{"", "text/plain", "text/html"} {
		t.Run(contentType, func(t *testing.T) {
			store, mock := newTestStore(t)
			message := email.Message{
				From: "from@example.com", To: "to@example.com", Subject: "subject", Body: "<p>body</p>",
				ContentType: contentType,
			}
			wantContentType := contentType
			if wantContentType == "" {
				wantContentType = "text/plain"
			}
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `records` (`from`,`to`,`subject`,`body`,`content_type`,`status`,`sent_at`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?,?,?,?)")).
				WithArgs(message.From, message.To, message.Subject, message.Body, wantContentType, int(email.StatusWaiting), nil, nonzeroTime{}, nonzeroTime{}).
				WillReturnResult(sqlmock.NewResult(42, 1))
			mock.ExpectCommit()
			if _, err := store.Create(context.Background(), message, email.StatusWaiting); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpdateStatusPersistsZeroAndOnlyWritesSentTimeForSentEmail(t *testing.T) {
	for _, status := range []email.Status{email.StatusFailed, email.StatusSent, email.StatusWaiting} {
		store, mock := newTestStore(t)
		var sentAt driver.Value
		if status == email.StatusSent {
			sentAt = nonzeroTime{}
		}
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("UPDATE `records` SET `status`=?,`updated_at`=?,`sent_at`=? WHERE `records`.`id` = ?")).
			WithArgs(int(status), nonzeroTime{}, sentAt, 42).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		if err := store.UpdateStatus(context.Background(), 42, status); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordStorePropagatesContextCancellation(t *testing.T) {
	store, _ := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Create(ctx, email.Message{}, email.StatusWaiting); !errors.Is(err, context.Canceled) {
		t.Fatalf("create returned %v, want context.Canceled", err)
	}
	if err := store.UpdateStatus(ctx, 42, email.StatusFailed); !errors.Is(err, context.Canceled) {
		t.Fatalf("update returned %v, want context.Canceled", err)
	}
}

func TestUpdateStatusReportsMissingRecord(t *testing.T) {
	store, mock := newTestStore(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `records`").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := store.UpdateStatus(context.Background(), 42, email.StatusFailed); err == nil {
		t.Fatal("update reported success without updating a record")
	}
}

func TestRecordStorePreservesDatabaseErrors(t *testing.T) {
	store, mock := newTestStore(t)
	wantErr := errors.New("database unavailable")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `records`").WillReturnError(wantErr)
	mock.ExpectRollback()
	if _, err := store.Create(context.Background(), email.Message{}, email.StatusWaiting); !errors.Is(err, wantErr) {
		t.Fatalf("create returned %v, want %v", err, wantErr)
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `records`").WillReturnError(wantErr)
	mock.ExpectRollback()
	if err := store.UpdateStatus(context.Background(), 42, email.StatusFailed); !errors.Is(err, wantErr) {
		t.Fatalf("update returned %v, want %v", err, wantErr)
	}
}
