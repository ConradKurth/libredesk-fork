package conversation

import (
	"testing"

	models "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/volatiletech/null/v9"
)

// assistantUsers returns an isAssistant predicate treating the given ids as AI
// assistant identity users.
func assistantUsers(ids ...int) func(int) bool {
	set := make(map[int]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return func(id int) bool { return set[id] }
}

func convAssignedTo(userID int) models.Conversation {
	return models.Conversation{AssignedUserID: null.IntFrom(userID)}
}

func TestAssistantHandOffApplies(t *testing.T) {
	const (
		assistantUser = 10 // AI assistant identity user (owns the conversation)
		humanUser     = 20 // human teammate replying
	)
	isAssistant := assistantUsers(assistantUser)

	tests := []struct {
		name    string
		message models.Message
		conv    models.Conversation
		want    bool
	}{
		{
			name:    "human reply while assistant owns it -> hand off",
			message: models.Message{SenderID: humanUser},
			conv:    convAssignedTo(assistantUser),
			want:    true,
		},
		{
			name:    "assistant's own reply -> no hand off",
			message: models.Message{SenderID: assistantUser},
			conv:    convAssignedTo(assistantUser),
			want:    false,
		},
		{
			name:    "automated integration reply -> no hand off",
			message: models.Message{SenderID: humanUser, Meta: []byte(`{"is_automated":true}`)},
			conv:    convAssignedTo(assistantUser),
			want:    false,
		},
		{
			name:    "human reply while a human already owns it -> no hand off",
			message: models.Message{SenderID: humanUser},
			conv:    convAssignedTo(humanUser),
			want:    false,
		},
		{
			name:    "human reply while unassigned -> no hand off",
			message: models.Message{SenderID: humanUser},
			conv:    models.Conversation{}, // AssignedUserID invalid
			want:    false,
		},
		{
			name:    "private note from human -> no hand off",
			message: models.Message{SenderID: humanUser, Private: true},
			conv:    convAssignedTo(assistantUser),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assistantHandOffApplies(tt.message, tt.conv, isAssistant); got != tt.want {
				t.Fatalf("assistantHandOffApplies = %v, want %v", got, tt.want)
			}
		})
	}
}
