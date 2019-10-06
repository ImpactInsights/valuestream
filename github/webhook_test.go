package github

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhook_Handler_InvalidGithubEvent(t *testing.T) {
	tracer := &StubTracer{}
	webhook := NewWebhook(tracer, nil)

	payload := []byte(`
{
	"result": "INPROGRESS",
	"buildUrl": "aUrl"
}`)

	req, err := http.NewRequest("GET", "/github", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
}

func TestWebhook_Handler_handleEventError(t *testing.T) {
	event := []byte(
		`{
		"action": "closed",
		"issue": {
            "number": 1
		},
		"repository": {
			"name": "valuestream"
        }
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")

	rr := httptest.NewRecorder()

	tracer := &StubTracer{
		ReturnValue: fmt.Errorf("error"),
	}
	webhook := NewWebhook(tracer, nil)
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 1, tracer.calls)
}

func TestWebhook_Handler_handleEventSuccess(t *testing.T) {
	event := []byte(
		`{
		"action": "closed",
		"issue": {
            "number": 1
		},
		"repository": {
			"name": "valuestream"
        }
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")

	rr := httptest.NewRecorder()

	tracer := &StubTracer{}
	webhook := NewWebhook(tracer, nil)
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 1, tracer.calls)
}

func TestWebhook_payload_GlobalSecretToken(t *testing.T) {
	event := []byte(
		`{
		"action": "closed",
		"issue": {
            "number": 1
		},
		"repository": {
			"name": "valuestream"
        }
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")
	req.Header.Set("Content-type", "application/json")

	webhook := NewWebhook(&StubTracer{}, []byte("secret"))
	_, err = webhook.payload(req, webhook.secretToken)
	assert.EqualError(t, err, "missing signature")
}

func TestWebhook_payload_RequestSecretToken(t *testing.T) {
	event := []byte(
		`{
		"action": "closed",
		"issue": {
            "number": 1
		},
		"repository": {
			"name": "valuestream"
        }
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")
	req.Header.Set("Content-type", "application/json")
	ctx := context.WithValue(req.Context(), CtxSecretTokenKey, []byte("secret"))
	req = req.WithContext(ctx)

	webhook := NewWebhook(&StubTracer{}, nil)
	_, err = webhook.payload(req, webhook.secretToken)
	assert.EqualError(t, err, "missing signature")
}

func TestWebhook_payload_NoSecretToken(t *testing.T) {
	event := []byte(
		`{
		"action": "closed",
		"issue": {
            "number": 1
		},
		"repository": {
			"name": "valuestream"
        }
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")
	req.Header.Set("Content-type", "application/json")

	webhook := NewWebhook(&StubTracer{}, nil)
	_, err = webhook.payload(req, webhook.secretToken)
	assert.NoError(t, err)
}
