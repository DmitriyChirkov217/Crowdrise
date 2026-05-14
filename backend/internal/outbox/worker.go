package outbox

import (
	"context"
	"time"

	"crowdrise/backend/internal/repositories"

	"github.com/rs/zerolog/log"
)

type Worker struct {
	repo     *repositories.Repository
	interval time.Duration
}

func New(repo *repositories.Repository, interval time.Duration) *Worker {
	return &Worker{repo: repo, interval: interval}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	items, err := w.repo.TakePendingOutbox(ctx, 20)
	if err != nil {
		log.Error().Err(err).Msg("outbox fetch failed")
		return
	}
	for _, item := range items {
		log.Info().Str("outbox_id", item.ID).Str("user_id", item.UserID).Str("event_type", item.EventType).RawJSON("payload", item.Payload).Msg("notification sent")
		if err := w.repo.MarkOutboxSent(ctx, item.ID); err != nil {
			log.Error().Err(err).Str("outbox_id", item.ID).Msg("outbox mark sent failed")
		}
	}
}
