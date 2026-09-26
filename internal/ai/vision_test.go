package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abhinavxd/libredesk/internal/ai/models"
	"github.com/zerodha/logf"
)

func TestDescribeImageUsesVisionModel(t *testing.T) {
	reply := "TEXT: Order #1234, Mint x2\nDESCRIPTION: a packing slip"
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		out, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": reply}}}})
		w.Write(out)
	}))
	defer srv.Close()

	lo := logf.New(logf.Opts{})
	// The completion model is text-only (Vision false); the image must still reach the vision model.
	cfg := models.ProviderConfig{BaseURL: srv.URL, APIKey: "test", Model: "text-model", VisionModel: "vision-model", ReasoningEffort: "low"}
	client := NewOpenAIClient(visionConfig(cfg), &lo, srv.Client())
	img := models.ChatImage{MediaType: "image/jpeg", Data: "aGVsbG8="}

	got, err := describeImage(context.Background(), client, img)
	if err != nil {
		t.Fatalf("describeImage: %v", err)
	}
	if got != reply {
		t.Errorf("describeImage = %q, want %q", got, reply)
	}
	if body["model"] != "vision-model" {
		t.Errorf("model = %v, want vision-model", body["model"])
	}
	if _, ok := body["reasoning_effort"]; ok {
		t.Error("the completion model's reasoning_effort must not be sent to the vision model")
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	parts, _ := msgs[1].(map[string]any)["content"].([]any)
	var sawImage bool
	for _, p := range parts {
		if part, _ := p.(map[string]any); part["type"] == "image_url" {
			url, _ := part["image_url"].(map[string]any)["url"].(string)
			sawImage = strings.HasPrefix(url, "data:image/jpeg;base64,")
		}
	}
	if !sawImage {
		t.Errorf("user message has no image_url part: %v", msgs[1])
	}

	for _, decorative := range []string{"DECORATIVE", " DECORATIVE.\n", "`DECORATIVE`"} {
		reply = decorative
		got, err := describeImage(context.Background(), client, img)
		if err != nil || got != "" {
			t.Errorf("reply %q: describeImage = %q, %v; want empty, nil", decorative, got, err)
		}
	}

	reply = "  "
	if _, err := describeImage(context.Background(), client, img); err == nil {
		t.Error("an empty reply must be an error so it is not cached as a description")
	}
}
