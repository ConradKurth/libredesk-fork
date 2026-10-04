package inbox

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/zerodha/logf"
)

type fakeInbox struct {
	id      int
	running atomic.Int32
	starts  atomic.Int32
}

func (f *fakeInbox) Close() error                      { return nil }
func (f *fakeInbox) Identifier() int                   { return f.id }
func (f *fakeInbox) Send(models.OutboundMessage) error { return nil }
func (f *fakeInbox) Name() string                      { return "fake" }
func (f *fakeInbox) FromAddress() string               { return "" }
func (f *fakeInbox) FromNameTemplate() string          { return "" }
func (f *fakeInbox) ReplyToAddress() string            { return "" }
func (f *fakeInbox) Channel() string                   { return "fake" }
func (f *fakeInbox) Receive(ctx context.Context) error {
	f.starts.Add(1)
	f.running.Add(1)
	defer f.running.Add(-1)
	<-ctx.Done()
	return nil
}

func newTestManager(inb Inbox) *Manager {
	lo := logf.New(logf.Opts{})
	return &Manager{
		lo:        &lo,
		inboxes:   map[int]Inbox{inb.Identifier(): inb},
		receivers: make(map[int]receiverState),
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRegisteredInboxesReceiveOnlyWhileStarted(t *testing.T) {
	inb := &fakeInbox{id: 1}
	m := newTestManager(inb)

	// Registered but not started (a follower): nothing receives, and a reload must not start a receiver.
	if m.activeReceiveCtx() != nil {
		t.Fatal("receive context active before Start")
	}

	// Leadership acquired.
	leader1, lose1 := context.WithCancel(context.Background())
	if err := m.Start(leader1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "receiver to start", func() bool { return inb.running.Load() == 1 })
	if m.activeReceiveCtx() == nil {
		t.Fatal("receive context inactive after Start")
	}

	// Leadership lost: receivers stop, the inbox stays registered, reloads stop starting receivers.
	lose1()
	waitFor(t, "receiver to stop", func() bool { return inb.running.Load() == 0 })
	if m.activeReceiveCtx() != nil {
		t.Fatal("receive context active after leadership loss")
	}
	if _, err := m.Get(1); err != nil {
		t.Fatalf("inbox unregistered after leadership loss: %v", err)
	}

	// Leadership re-acquired: exactly one receiver runs again.
	leader2, lose2 := context.WithCancel(context.Background())
	defer lose2()
	if err := m.Start(leader2); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "receiver to restart", func() bool { return inb.starts.Load() == 2 })
	if got := inb.running.Load(); got != 1 {
		t.Fatalf("running receivers = %d, want 1", got)
	}
}
