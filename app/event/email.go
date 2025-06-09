package event

import (
	"context"

	"github.com/miebyte/emailsender/domain/email"
	"github.com/miebyte/emailsender/domain/record"
)

type EmailEventHandler struct {
	recordService record.Service
}

func NewEmailEventHandler(recordService record.Service) email.EventHandler {
	return &EmailEventHandler{
		recordService: recordService,
	}
}

func (h *EmailEventHandler) HandleEmailSent(event email.EmailSentEvent) error {
	record := &record.EmailSendRecord{
		ID:     event.ID,
		Status: record.SendStatus(event.Status),
	}

	return h.recordService.UpdateRecord(context.Background(), record)
}
