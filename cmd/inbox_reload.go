package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/abhinavxd/libredesk/internal/ws/redisbackplane"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/zerodha/logf"
)

// inboxReloadPublishTimeout bounds a single publish so a stalled Redis can't hang the
// HTTP handler that changed the inbox.
const inboxReloadPublishTimeout = 5 * time.Second

// inboxReloadRelay tells every instance to reload an inbox after it changes. Each instance
// holds its own initialized copy of every inbox, so an edit handled by one replica must
// reach the others too, notably the leader, which runs the receivers (IMAP polling).
type inboxReloadRelay struct {
	backplane *redisbackplane.Backplane
	origin    string
	lo        *logf.Logger
}

type inboxReloadEnvelope struct {
	// Origin is the publisher's instance ID; it already reloaded the inbox locally.
	Origin  string `json:"origin"`
	InboxID int    `json:"inbox_id"`
}

func initInboxReloadRelay(rdb *redis.Client) *inboxReloadRelay {
	lo := initLogger("inbox-reload-relay")
	return &inboxReloadRelay{backplane: redisbackplane.New(rdb, inboxReloadChannel, lo), origin: uuid.NewString(), lo: lo}
}

// publish asks the other instances to reload the inbox. A failure is logged: this instance
// already reloaded, and the others pick the change up on their next restart.
func (r *inboxReloadRelay) publish(inboxID int) {
	payload, err := json.Marshal(inboxReloadEnvelope{Origin: r.origin, InboxID: inboxID})
	if err != nil {
		r.lo.Error("marshalling inbox reload envelope failed", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), inboxReloadPublishTimeout)
	defer cancel()
	if err := r.backplane.Publish(ctx, payload); err != nil {
		r.lo.Error("publishing inbox reload failed", "inbox_id", inboxID, "error", err)
	}
}

// consume reloads inboxes changed on other instances until ctx is cancelled.
func (r *inboxReloadRelay) consume(ctx context.Context, app *App) {
	ch, err := r.backplane.Subscribe(ctx)
	if err != nil {
		r.lo.Error("subscribing to inbox reloads failed", "error", err)
		return
	}
	for payload := range ch {
		var env inboxReloadEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			r.lo.Error("unmarshalling inbox reload envelope failed", "error", err)
			continue
		}
		if env.Origin == r.origin {
			continue
		}
		if err := reloadInboxLocal(app, env.InboxID); err != nil {
			r.lo.Error("error reloading inbox changed on another instance", "inbox_id", env.InboxID, "error", err)
		}
	}
}
