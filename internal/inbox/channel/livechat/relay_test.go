package livechat

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/zerodha/logf"
)

// memBus is an in-memory Backplane that delivers every payload to all subscribers,
// including the publisher, like Redis pub/sub.
type memBus struct {
	mu   sync.Mutex
	subs []chan []byte
}

func (b *memBus) Publish(_ context.Context, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		ch <- payload
	}
	return nil
}

func (b *memBus) Subscribe(ctx context.Context) (<-chan []byte, error) {
	ch := make(chan []byte, 64)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, c := range b.subs {
			if c == ch {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				close(ch)
				break
			}
		}
	}()
	return ch, nil
}

type stubUsers struct{}

func (stubUsers) GetAgent(id int, _ string) (umodels.User, error) {
	return umodels.User{ID: id, FirstName: "Agent"}, nil
}
func (stubUsers) IsEmailBlocked(string) (bool, error) { return false, nil }

// newInstance builds one replica's copy of live chat inbox 1, wired to the shared bus.
func newInstance(t *testing.T, ctx context.Context, bus *memBus) *LiveChat {
	t.Helper()
	lo := logf.New(logf.Opts{})
	var lc *LiveChat
	relay := NewRelay(bus, &lo, func(id int) *LiveChat {
		if id == 1 {
			return lc
		}
		return nil
	})
	var err error
	lc, err = New(nil, stubUsers{}, Opts{ID: 1, Lo: &lo, Relay: relay})
	if err != nil {
		t.Fatal(err)
	}
	go relay.Consume(ctx)
	return lc
}

func waitSubscribers(t *testing.T, bus *memBus, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		bus.mu.Lock()
		got := len(bus.subs)
		bus.mu.Unlock()
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers = %d, want %d", got, n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// received drains a client's channel for a short window and returns how many payloads arrived.
func received(c *Client) int {
	n := 0
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case <-c.Channel:
			n++
		case <-timeout:
			return n
		}
	}
}

func TestRelayDeliversToClientOnAnotherInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := &memBus{}
	podA := newInstance(t, ctx, bus)
	podB := newInstance(t, ctx, bus)
	waitSubscribers(t, bus, 2)

	// The contact's widget is connected to pod B; the reply is sent from pod A (the leader).
	client, err := podB.AddClient("7", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := podA.Send(models.OutboundMessage{MessageReceiverID: 7, SenderID: 3, UUID: "m1", ConversationUUID: "c1"}); err != nil {
		t.Fatalf("Send with a relay and no local client = %v, want nil", err)
	}
	podA.BroadcastTypingToClients("c1", 7, true)
	podA.BroadcastConversationToClients("c1", 7, map[string]any{"status": "Open"})
	podA.BroadcastMessageToClients("c1", 7, map[string]any{"uuid": "m2"})

	if got := received(client); got != 4 {
		t.Fatalf("pod B client received %d payloads, want 4", got)
	}
}

func TestRelayDeliversOncePerClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := &memBus{}
	podA := newInstance(t, ctx, bus)
	podB := newInstance(t, ctx, bus)
	waitSubscribers(t, bus, 2)

	// The contact has the widget open on two devices, one on each instance.
	onA, _ := podA.AddClient("7", nil)
	onB, _ := podB.AddClient("7", nil)
	other, _ := podB.AddClient("8", nil)

	podA.BroadcastMessageToClients("c1", 7, map[string]any{"uuid": "m1"})

	if got := received(onA); got != 1 {
		t.Fatalf("origin client received %d payloads, want 1 (no echo from the relay)", got)
	}
	if got := received(onB); got != 1 {
		t.Fatalf("remote client received %d payloads, want 1", got)
	}
	if got := received(other); got != 0 {
		t.Fatalf("another contact's client received %d payloads, want 0", got)
	}
}

func TestSendWithoutRelayReportsNotConnected(t *testing.T) {
	lo := logf.New(logf.Opts{})
	lc, _ := New(nil, stubUsers{}, Opts{ID: 1, Lo: &lo})
	if err := lc.Send(models.OutboundMessage{MessageReceiverID: 7, SenderID: 3}); err != ErrClientNotConnected {
		t.Fatalf("Send = %v, want ErrClientNotConnected", err)
	}
}
