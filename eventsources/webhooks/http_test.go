package webhooks

import (
	"bytes"
	"context"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/tracers"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
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

func TestWebhook_handleEndEvent_WithTrace_Success(t *testing.T) {
	tracer := mocktracer.New()

	wh := &Webhook{
		Traces: traces.NewMemoryUnboundedSpanStore(),
		Spans:  traces.NewMemoryUnboundedSpanStore(),
		EventSource: eventsources.StubEventSource{
			TracerReturn: tracer,
		},
	}

	spanID := "span-test-1"
	traceID := "trace-test-1"

	e := eventsources.StubEvent{
		SpanIDReturn:  spanID,
		TraceIDReturn: &traceID,
	}

	span := tracer.StartSpan("test_operation")
	wh.Spans.Set(
		context.Background(),
		spanID,
		traces.StoreEntry{Span: span},
	)

	assert.Nil(t, wh.handleEndEvent(
		context.Background(),
		tracer,
		e,
	))

	numTraces, _ := wh.Traces.Count()
	assert.Equal(t, 0, numTraces)

	numSpans, _ := wh.Spans.Count()
	assert.Equal(t, 0, numSpans)
}

func TestWebhook_handleStartEvent_WithTrace_Success(t *testing.T) {
	tracer := mocktracer.New()

	wh := &Webhook{
		Traces: traces.NewMemoryUnboundedSpanStore(),
		Spans:  traces.NewMemoryUnboundedSpanStore(),
		EventSource: eventsources.StubEventSource{
			TracerReturn: tracer,
		},
	}

	spanID := "span-test-1"
	traceID := "trace-test-1"

	e := eventsources.StubEvent{
		OperationNameReturn: "operation_name",
		SpanIDReturn:        spanID,
		TraceIDReturn:       &traceID,
	}

	assert.Nil(t, wh.handleStartEvent(
		context.Background(),
		tracer,
		e,
	))

	numTraces, _ := wh.Traces.Count()
	assert.Equal(t, 1, numTraces)

	numSpans, _ := wh.Spans.Count()
	assert.Equal(t, 1, numSpans)

	entry, err := wh.Spans.Get(context.Background(), tracer, spanID)
	assert.NoError(t, err)
	assert.Equal(t, e.OperationName(), entry.Span.(*mocktracer.MockSpan).OperationName)

	entry, err = wh.Traces.Get(context.Background(), tracer, traceID)
	assert.NoError(t, err)
	assert.Equal(t, e.OperationName(), entry.Span.(*mocktracer.MockSpan).OperationName)
}

func TestWebhook_Handler_Success(t *testing.T) {
	req, err := http.NewRequest(
		"GET",
		"/test",
		bytes.NewReader([]byte(`throwaway`)),
	)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()

	wh := &Webhook{
		Tracers: tracers.NewRequestScopedUsingSources(),
		EventSource: eventsources.StubEventSource{
			ValidatePayloadFn: func(r *http.Request, secretKey []byte) ([]byte, error) {
				return nil, nil
			},
			EventFn: func(*http.Request, []byte) (eventsources.Event, error) {
				return eventsources.StubEvent{
					StateReturn:      eventsources.IntermediaryState,
					StateReturnError: nil,
				}, nil
			},
		},
	}
	wh.Handler(rr, req)
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
}
