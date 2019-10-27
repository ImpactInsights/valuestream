package webhooks

import (
	"context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestWebhook_secretKey(t *testing.T) {
	testCases := []struct {
		name         string
		whSecretKey  []byte
		ctxSecretKey []byte
		expected     []byte
	}{
		{
			"no_webhook_secret_no_request_secret",
			nil,
			nil,
			nil,
		},
		{
			"webhook_secret",
			[]byte("webhook_secret"),
			nil,
			[]byte("webhook_secret"),
		},
		{
			"request_scoped_secret",
			nil,
			[]byte("webhook_secret"),
			[]byte("webhook_secret"),
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			wh := Webhook{SecretKey: tt.whSecretKey}
			req, err := http.NewRequest("GET", "/test", nil)
			assert.NoError(t, err)

			if tt.ctxSecretKey != nil {
				ctx := context.WithValue(
					req.Context(),
					CtxSecretTokenKey,
					tt.ctxSecretKey,
				)

				req = req.WithContext(ctx)
			}
			assert.Equal(t,
				tt.expected,
				wh.secretKey(req),
			)
		})
	}
}

func TestWebhook_handleEvent(t *testing.T) {
	t.Fail()
}

func TestWebhook_Handler(t *testing.T) {
	t.Fail()
}
