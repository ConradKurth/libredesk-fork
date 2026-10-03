package aiagent

import (
	"regexp"
	"strings"
)

var (
	// "A teammate will process the replacement", "our support team will follow up", "someone from
	// our team will reach out".
	teammateWillActRe = regexp.MustCompile(`(?i)\b(?:teammates?|team members?|(?:our|the) (?:support |customer care |customer service )?team|someone(?: from (?:our|the) (?:support )?team)?|a (?:human|person|specialist|representative|rep|support agent))\b[^.?!]{0,30}?(?:\bwill\b|['’]ll\b|\b(?:is|are) going to\b)[^.?!]{0,20}?\b(?:follow up|reach out|get back|contact|process|arrange|review|confirm|send|ship|look into|handle|take care|take over|be in touch|check|issue|refund|cancel|update)\b`)
	// "We'll arrange a replacement", "we will send the missing items".
	weWillFixRe = regexp.MustCompile(`(?i)\bwe(?:['’]ll| will| are going to)\b[^.?!]{0,20}?\b(?:arrange|process|send|ship|issue|refund)\b[^.?!]{0,30}?\b(?:replacement|refund|reship|return label|missing|credit)`)
	// "Once we have the photo, a teammate will arrange a replacement": a next step that waits on the
	// customer, said while the assistant is still collecting details.
	conditionalRe = regexp.MustCompile(`(?i)\b(?:once|if|after|when|whenever|as soon as|then|let (?:us|me) know)\b`)
)

// claimsTeammateAction reports whether a reply promises the customer that a person on our side
// will act now (process a replacement, follow up, issue a refund). Questions and conditional next
// steps are skipped: "would you like a teammate to follow up?" asks, and "once we have the photo, a
// teammate will..." waits on the customer.
func claimsTeammateAction(reply string) bool {
	for _, sentence := range sentenceRe.FindAllString(reply, -1) {
		if strings.HasSuffix(strings.TrimSpace(sentence), "?") || conditionalRe.MatchString(sentence) {
			continue
		}
		if teammateWillActRe.MatchString(sentence) || weWillFixRe.MatchString(sentence) {
			return true
		}
	}
	return false
}
