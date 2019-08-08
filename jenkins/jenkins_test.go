package jenkins

import (
	"bytes"
	"context"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEventTracer_HTTPBuildHandler_StartTrace(t *testing.T) {
	payload := []byte(`
{
	"result": "INPROGRESS",
	"buildUrl": "aUrl"
}`)

	req, err := http.NewRequest("GET", "/jenkins", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	tracer := mocktracer.New()
	et := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	webhook := NewWebhook(et)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.HTTPBuildHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	c, _ := et.spans.Count()
	assert.Equal(t, 1, c)
}

func TestEventTracer_HTTPBuildHandler_EndTrace(t *testing.T) {
	payload := []byte(`
{
	"result": "ABORTED",
	"buildUrl": "aUrl"
}`)

	req, err := http.NewRequest("GET", "/jenkins", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	tracer := mocktracer.New()
	et := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	webhook := NewWebhook(et)

	et.spans.Set(
		context.Background(),
		"aUrl",
		tracer.StartSpan("build"),
	)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.HTTPBuildHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	c, _ := et.spans.Count()
	assert.Equal(t, 0, c)
}

func TestEventTracer_HTTPBuildHandler_MissingStartEvent(t *testing.T) {
	payload := []byte(`
{
	"result": "ABORTED",
	"buildUrl": "aUrl"
}`)

	req, err := http.NewRequest("GET", "/jenkins", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	tracer := mocktracer.New()
	et := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	webhook := NewWebhook(et)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.HTTPBuildHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	c, _ := et.spans.Count()
	assert.Equal(t, 0, c)
}
