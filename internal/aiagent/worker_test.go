package aiagent

import (
	"reflect"
	"testing"
)

func TestSplitSuggestions(t *testing.T) {
	body, suggestions := splitSuggestions("Which plan?\n[[suggestions]] [\"Basic\",\"Pro\",\"Enterprise\",\"Extra\"]")
	if body != "Which plan?" {
		t.Fatalf("unexpected body: %q", body)
	}
	want := []string{"Basic", "Pro", "Enterprise"}
	if !reflect.DeepEqual(suggestions, want) {
		t.Fatalf("unexpected suggestions: %#v", suggestions)
	}
}

func TestSplitSuggestionsRejectsInvalidPayload(t *testing.T) {
	body, suggestions := splitSuggestions("Tell me more\n[[suggestions]] not-json")
	if body != "Tell me more" || suggestions != nil {
		t.Fatalf("unexpected result: %q %#v", body, suggestions)
	}
}

func TestSplitSuggestionsBareArray(t *testing.T) {
	cases := []struct{ in, body string }{
		// Marker dropped (email to a customer, 2026-10-23).
		{"Contact UPS.\n\nPlease reply to this email.\n[\"Yes, that helps\",\"I need more assistance\"]", "Contact UPS.\n\nPlease reply to this email."},
		// Opening bracket lost (widget, CYC-1689).
		{"\"Yes, thanks\",\"No, I need more help\"]", ""},
		{"Did that resolve your question?,\n[\"Yes\",\"No\"]", "Did that resolve your question?"},
	}
	for _, c := range cases {
		body, suggestions := splitSuggestions(c.in)
		if body != c.body || len(suggestions) != 2 {
			t.Fatalf("%q: got body %q suggestions %#v", c.in, body, suggestions)
		}
	}
}

func TestSplitSuggestionsKeepsOrdinaryText(t *testing.T) {
	for _, in := range []string{
		"Your order shipped.",
		"The label reads \"Mint\", \"Lush Ice\" on the side.",
		"Pick one:\n\"Mint\"",
	} {
		if body, suggestions := splitSuggestions(in); body != in || suggestions != nil {
			t.Fatalf("%q: got body %q suggestions %#v", in, body, suggestions)
		}
	}
}

func TestIsClosingAcknowledgement(t *testing.T) {
	cases := []struct {
		text      string
		afterConf bool
		want      bool
	}{
		{"Yes, thanks", false, true},
		{"Yes, that helps", false, true},
		{"Thank you for your assistance.", false, true},
		{"thx!", false, true},
		{"Yes", true, true},
		{"Yes", false, false},
		{"ok", true, false},
		{"Yes, I would.", false, false},
		{"Thanks, but where is my order?", false, false},
		{"No, I need more help", true, false},
		{"thanks, can you cancel it", false, false},
		{"", true, false},
	}
	for _, c := range cases {
		if got := isClosingAcknowledgement(c.text, c.afterConf); got != c.want {
			t.Fatalf("%q (afterConfirmation=%v): got %v, want %v", c.text, c.afterConf, got, c.want)
		}
	}
}
