package recordSrv

import (
	"context"

	"gitea.hoven.com/core/emailsender/domain/event"
	"gitea.hoven.com/core/emailsender/domain/record"
)

func (rs *recordService) UpdateRecordStatusEvent(ctx context.Context, evt event.Event) error {
	emailEvent, ok := evt.(*record.EmailStatusEvent)
	if !ok {
		return nil
	}

	return rs.recordRepo.UpdateStatus(ctx, emailEvent.ID, record.SendStatus(emailEvent.Status))
}
