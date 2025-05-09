package recordSrv

import (
	"context"

	"github.com/go-puzzles/emailsender/domain/event"
	"github.com/go-puzzles/emailsender/domain/record"
)

func (rs *recordService) UpdateRecordStatusEvent(ctx context.Context, evt event.Event) error {
	emailEvent, ok := evt.(*record.EmailStatusEvent)
	if !ok {
		return nil
	}

	return rs.recordRepo.UpdateStatus(ctx, emailEvent.ID, record.SendStatus(emailEvent.Status))
}
