package livechat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/zerodha/logf"
)

// relayPublishTimeout bounds a single publish so a stalled backplane can't block
// the goroutine sending to the widget (often the outgoing message worker).
const relayPublishTimeout = 5 * time.Second

// Backplane relays payloads between Libredesk instances. It must deliver every
// published payload to all subscribed instances, including the publisher. Redis
// pub/sub (internal/ws/redisbackplane) is the reference implementation.
type Backplane interface {
	Publish(ctx context.Context, payload []byte) error
	Subscribe(ctx context.Context) (<-chan []byte, error)
}

// relayEnvelope is the wire format: a widget payload for one contact's clients
// on one live chat inbox.
type relayEnvelope struct {
	// Origin is the publisher's instance ID; receivers skip their own envelopes.
	Origin    string          `json:"origin"`
	InboxID   int             `json:"inbox_id"`
	ContactID string          `json:"contact_id"`
	Data      json.RawMessage `json:"data"`
}

// Relay fans widget payloads out to every instance. A contact's widget websocket
// may be connected to any replica, while the message for it is produced wherever
// the reply was sent, so each instance delivers locally and publishes, and every
// other instance replays the payload against its own connected clients.
type Relay struct {
	backplane Backplane
	origin    string
	lo        *logf.Logger
	// lookup returns this instance's live chat inbox for an ID, or nil.
	lookup func(inboxID int) *LiveChat
}

// NewRelay returns a relay over the backplane. lookup resolves an inbox ID to this
// instance's live chat inbox when replaying payloads from other instances.
func NewRelay(backplane Backplane, lo *logf.Logger, lookup func(inboxID int) *LiveChat) *Relay {
	return &Relay{backplane: backplane, origin: uuid.NewString(), lo: lo, lookup: lookup}
}

// publish relays a payload to other instances. Failures are logged and swallowed:
// the payload was already delivered locally, and the widget re-syncs from the
// database when it reconnects.
func (r *Relay) publish(inboxID int, contactID string, data []byte) {
	payload, err := json.Marshal(relayEnvelope{Origin: r.origin, InboxID: inboxID, ContactID: contactID, Data: data})
	if err != nil {
		r.lo.Error("marshalling widget relay envelope failed", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), relayPublishTimeout)
	defer cancel()
	if err := r.backplane.Publish(ctx, payload); err != nil {
		r.lo.Error("publishing to widget relay failed", "inbox_id", inboxID, "error", err)
	}
}

// Consume blocks and delivers payloads published by other instances to locally
// connected widget clients until ctx is cancelled. Run it in its own goroutine.
func (r *Relay) Consume(ctx context.Context) {
	ch, err := r.backplane.Subscribe(ctx)
	if err != nil {
		r.lo.Error("subscribing to widget relay failed", "error", err)
		return
	}
	for payload := range ch {
		var env relayEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			r.lo.Error("unmarshalling widget relay envelope failed", "error", err)
			continue
		}
		// The origin already delivered this locally.
		if env.Origin == r.origin {
			continue
		}
		if lc := r.lookup(env.InboxID); lc != nil {
			lc.deliverLocal(env.ContactID, env.Data)
		}
	}
}
