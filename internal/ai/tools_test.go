package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abhinavxd/libredesk/internal/ai/models"
	"github.com/zerodha/logf"
)

func TestHTTPToolHandoffHeader(t *testing.T) {
	cases := []struct {
		name   string
		header string
		status int
		want   []string
	}{
		{"queued for a human", "Replacement proposed for a teammate", http.StatusOK, []string{"Replacement proposed for a teammate"}},
		{"no header", "", http.StatusOK, nil},
		{"failed call", "ignored", http.StatusBadGateway, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.header != "" {
					w.Header().Set(handoffHeader, tc.header)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"ok"}`))
			}))
			defer srv.Close()

			lo := logf.New(logf.Opts{})
			var got []string
			tool := newHTTPTool(models.Tool{Name: "propose", URL: srv.URL, Method: http.MethodPost}, "", &lo, srv.Client(),
				ToolContext{OnHandoff: func(reason string) { got = append(got, reason) }})
			if _, err := tool.Execute(context.Background(), `{}`); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Errorf("OnHandoff calls = %q, want %q", got, tc.want)
			}
		})
	}
}
