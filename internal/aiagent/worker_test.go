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
		{"Yes, it did", true, true},
		{"Yes, it does.", true, true},
		{"Yes, thanks!", false, true},
		{"It did not", true, false},
	}
	for _, c := range cases {
		if got := isClosingAcknowledgement(c.text, c.afterConf); got != c.want {
			t.Fatalf("%q (afterConfirmation=%v): got %v, want %v", c.text, c.afterConf, got, c.want)
		}
	}
}

func TestTrimBlank(t *testing.T) {
	for _, in := range []string{"\u200b", " \u200b\n", "\ufeff\u200d", ""} {
		if got := trimBlank(in); got != "" {
			t.Fatalf("%q: got %q, want empty", in, got)
		}
	}
	if got := trimBlank("\u200b Hi there.\n"); got != "Hi there." {
		t.Fatalf("got %q", got)
	}
}

func TestCleanReply(t *testing.T) {
	cases := map[string]string{
		"<|start|>assistant<|channel|>final**Gust Core Mint is not menthol.**\n\nDid that answer your question?<|return|>": "**Gust Core Mint is not menthol.**\n\nDid that answer your question?",
		"Use our store locator: **cyclonepods.com/pages/store\u2011locator**.":                                             "Use our store locator: **cyclonepods.com/pages/store-locator**.",
		"See https://cyclonepods.com/pages/store\u2011locator\u202f\u2013 thanks":                                          "See https://cyclonepods.com/pages/store-locator\u202f\u2013 thanks",
		"A mint\u2011style flavor, non\u2011ice. Email support@cyclonepods.com.":                                           "A mint\u2011style flavor, non\u2011ice. Email support@cyclonepods.com.",
		"\u200b": "",
	}
	for in, want := range cases {
		if got := cleanReply(in); got != want {
			t.Errorf("cleanReply(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSpecialTokenSurvivesCleanReply(t *testing.T) {
	if specialToken.MatchString(cleanReply("<|start|>assistant<|channel|>final Thanks!<|return|>")) {
		t.Fatal("known harmony markers are stripped, not treated as garbled output")
	}
	if !specialToken.MatchString(cleanReply("uid Bill and.\"// <|reserved_200016|>--- (.,//.")) {
		t.Fatal("expected a leftover tokenizer marker to be detected")
	}
	if specialToken.MatchString(cleanReply("Use code <b>CYCLONE_10</b> at checkout.")) {
		t.Fatal("ordinary text flagged as garbled")
	}
}
