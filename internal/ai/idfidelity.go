package ai

import (
	"regexp"

	"github.com/abhinavxd/libredesk/internal/ai/models"
)

// idTokenRe matches identifier-like tokens: tracking numbers, order references, and the like (also
// inside URLs such as ...?tLabels=9300...). Only tokens with at least minIDDigits digits count, so
// ordinary words and short order numbers are never touched.
var idTokenRe = regexp.MustCompile(`\b[0-9A-Z]{10,}\b`)

const minIDDigits = 8

// idRepair records one identifier the model mis-copied and what it was corrected to.
type idRepair struct {
	From, To string
}

// repairCopiedIDs corrects identifiers the model garbled while copying them out of tool results.
// Models drop or swap digits in long numbers (a 22-digit USPS number came back 3 digits short), and
// a wrong tracking link is worse than none. Every identifier in the answer must appear verbatim in a
// tool result or a user message; one that doesn't is replaced by the nearest such source identifier
// when it is a near-copy of it. Anything else (no close source, or a tie) is left as is.
func repairCopiedIDs(answer string, messages []models.ChatMessage) (string, []idRepair) {
	sources := map[string]bool{}
	for _, msg := range messages {
		if msg.Role != models.RoleTool && msg.Role != models.RoleUser {
			continue
		}
		for _, tok := range idTokens(msg.Content) {
			sources[tok] = true
		}
	}
	if len(sources) == 0 {
		return answer, nil
	}

	var repairs []idRepair
	seen := map[string]bool{}
	for _, tok := range idTokens(answer) {
		if sources[tok] || seen[tok] {
			continue
		}
		seen[tok] = true
		if fix, ok := nearestID(tok, sources); ok {
			repairs = append(repairs, idRepair{From: tok, To: fix})
		}
	}
	for _, r := range repairs {
		answer = replaceIDToken(answer, r.From, r.To)
	}
	return answer, repairs
}

func idTokens(s string) []string {
	var out []string
	for _, tok := range idTokenRe.FindAllString(s, -1) {
		digits := 0
		for _, c := range tok {
			if c >= '0' && c <= '9' {
				digits++
			}
		}
		if digits >= minIDDigits {
			out = append(out, tok)
		}
	}
	return out
}

// nearestID returns the source identifier closest to tok by edit distance, if it is a near-copy
// (within a quarter of its length, at least 2) and unambiguous.
func nearestID(tok string, sources map[string]bool) (string, bool) {
	best, bestDist, tie := "", -1, false
	for src := range sources {
		d := levenshtein(tok, src)
		if d > max(2, len(src)/4) {
			continue
		}
		switch {
		case bestDist < 0 || d < bestDist:
			best, bestDist, tie = src, d, false
		case d == bestDist:
			tie = true
		}
	}
	return best, bestDist >= 0 && !tie
}

// replaceIDToken replaces whole-token occurrences of from, so a short token never rewrites part of
// a longer one.
func replaceIDToken(s, from, to string) string {
	return idTokenRe.ReplaceAllStringFunc(s, func(tok string) string {
		if tok == from {
			return to
		}
		return tok
	})
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// applyIDRepairs runs repairCopiedIDs on a final answer and logs each correction.
func (m *Manager) applyIDRepairs(answer string, messages []models.ChatMessage) string {
	fixed, repairs := repairCopiedIDs(answer, messages)
	for _, r := range repairs {
		m.lo.Warn("ai answer identifier repaired", "from", r.From, "to", r.To)
	}
	return fixed
}
