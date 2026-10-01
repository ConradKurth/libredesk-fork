package aiagent

import (
	"context"
	"testing"

	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/zerodha/logf"
)

// The handoff tool only records the handoff, so the worker can post the run's answer before transferring.
// A Manager with no conversation store would panic if Execute tried to transfer.
func TestHandoffToolRecordsOnly(t *testing.T) {
	lo := logf.New(logf.Opts{})
	outcome := &runOutcome{}
	tool := &handoffTool{m: &Manager{lo: &lo}, conv: cmodels.Conversation{}, outcome: outcome}

	out, err := tool.Execute(context.Background(), `{"reason":"Cancel subscription request"}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !outcome.handedOff {
		t.Error("handedOff = false, want true")
	}
	if outcome.handoffReason != "Cancel subscription request" {
		t.Errorf("handoffReason = %q", outcome.handoffReason)
	}
	if out == "" {
		t.Error("empty tool result; the model needs to be told to write its reply")
	}
}

func TestJoinNonEmpty(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"Answer.", "We've passed this on."}, "Answer.\n\nWe've passed this on."},
		{[]string{"", "We've passed this on."}, "We've passed this on."},
		{[]string{"  Answer.  ", " "}, "Answer."},
		{[]string{"", ""}, ""},
	}
	for _, c := range cases {
		if got := joinNonEmpty(c.parts...); got != c.want {
			t.Errorf("joinNonEmpty(%q) = %q, want %q", c.parts, got, c.want)
		}
	}
}
