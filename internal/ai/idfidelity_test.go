package ai

import (
	"testing"

	"github.com/abhinavxd/libredesk/internal/ai/models"
)

// The order_lookup result from conversation 270a9a6b: the model copied the 22-digit USPS number
// into its reply with "836" dropped, sending the customer a dead tracking link.
const orderLookupResult = `{"found":true,"orders":[{"name":"#453682","date":"2026-09-26","fulfillment_status":"SHIPPED","tracking":[{"number":"9300120787713608360397","url":"https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713608360397","company":"USPS"}]}]}`

func lookupHistory(toolResults ...string) []models.ChatMessage {
	msgs := []models.ChatMessage{
		{Role: models.RoleSystem, Content: "You are a support assistant."},
		{Role: models.RoleUser, Content: "Phone: 6153067525\n\nI haven't received my order? Was sent out 9/27?"},
	}
	for _, r := range toolResults {
		msgs = append(msgs, models.ChatMessage{Role: models.RoleTool, Name: "order_lookup", Content: r})
	}
	return msgs
}

func TestRepairCopiedIDs(t *testing.T) {
	tests := []struct {
		name    string
		answer  string
		tools   []string
		want    string
		repairs int
	}{
		{
			name:    "mis-copied tracking number in a URL",
			answer:  "Your order #453682 shipped. Track it here: <https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713600397>.",
			tools:   []string{orderLookupResult},
			want:    "Your order #453682 shipped. Track it here: <https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713608360397>.",
			repairs: 1,
		},
		{
			name:    "mis-copied bare number, every occurrence",
			answer:  "Tracking 9300120787713600397 (USPS): https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713600397",
			tools:   []string{orderLookupResult},
			want:    "Tracking 9300120787713608360397 (USPS): https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713608360397",
			repairs: 1,
		},
		{
			name:   "correct number unchanged",
			answer: "Track it here: https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713608360397",
			tools:  []string{orderLookupResult},
			want:   "Track it here: https://tools.usps.com/go/TrackConfirmAction?tLabels=9300120787713608360397",
		},
		{
			name:   "customer's own phone number unchanged",
			answer: "We'll call you at 6153067525 if needed.",
			tools:  []string{orderLookupResult},
			want:   "We'll call you at 6153067525 if needed.",
		},
		{
			name:   "far-off number left alone",
			answer: "Reference 1234567890123 is unrelated.",
			tools:  []string{orderLookupResult},
			want:   "Reference 1234567890123 is unrelated.",
		},
		{
			name:    "nearest of two tracking numbers",
			answer:  "Second box: 1Z999AA1012345678",
			tools:   []string{`{"tracking":[{"number":"1Z999AA10123456784"},{"number":"9400111899223456789012"}]}`},
			want:    "Second box: 1Z999AA10123456784",
			repairs: 1,
		},
		{
			name:   "no tool results, nothing to check against",
			answer: "Tracking 9300120787713600397",
			want:   "Tracking 9300120787713600397",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, repairs := repairCopiedIDs(tt.answer, lookupHistory(tt.tools...))
			if got != tt.want {
				t.Fatalf("got  %q\nwant %q", got, tt.want)
			}
			if len(repairs) != tt.repairs {
				t.Fatalf("got %d repairs %v, want %d", len(repairs), repairs, tt.repairs)
			}
		})
	}
}

// The model's own earlier replies are not a trusted source: a number it garbled in a previous turn
// must still be corrected against the tool result.
func TestRepairCopiedIDsIgnoresAssistantHistory(t *testing.T) {
	msgs := lookupHistory(orderLookupResult)
	msgs = append(msgs, models.ChatMessage{Role: models.RoleAssistant, Content: "Track: 9300120787713600397"})
	got, _ := repairCopiedIDs("Track: 9300120787713600397", msgs)
	if got != "Track: 9300120787713608360397" {
		t.Fatalf("got %q", got)
	}
}
