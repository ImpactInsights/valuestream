package jenkins

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhook_HTTPBuildHandler_InvalidPayload(t *testing.T) {
	payload := []byte(`
broken!#
	"huh_missing!": "aUrl"
}`)

	req, err := http.NewRequest("GET", "/jenkins", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	webhook := NewWebhook(&StubEventTracer{})
	handler := http.HandlerFunc(webhook.HTTPBuildHandler)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
}

func TestWebhook_HTTPBuildHandler_HandleBuildError(t *testing.T) {
	payload := []byte(`
{
	"huh_missing!": "aUrl",
	"can't'": "use"
}`)

	req, err := http.NewRequest("GET", "/jenkins", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	eventTracer := &StubEventTracer{
		ReturnValue: fmt.Errorf("error"),
	}
	webhook := NewWebhook(eventTracer)
	handler := http.HandlerFunc(webhook.HTTPBuildHandler)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Equal(t, 1, eventTracer.calls)
}

func TestWebhook_HTTPBuildHandler_HandleBuildSuccess(t *testing.T) {
	payload := []byte(`{}`)
	req, err := http.NewRequest("GET", "/jenkins", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	eventTracer := &StubEventTracer{
		ReturnValue: nil,
	}
	webhook := NewWebhook(eventTracer)
	handler := http.HandlerFunc(webhook.HTTPBuildHandler)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 1, eventTracer.calls)
}
