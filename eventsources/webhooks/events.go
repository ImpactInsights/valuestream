package webhooks

import (
	"github.com/opentracing/opentracing-go"
	"net/http"
)

type SpanState string

const (
	StartState        SpanState = "start"
	EndState          SpanState = "end"
	IntermediaryState SpanState = "intermediary"
	UnknownState      SpanState = "unknown"
)

const CtxSecretTokenKey = "secret_token"

type Event interface {
	SpanID() (string, error)
	OperationName() string
	ParentSpanID() (*string, error)
	IsError() (bool, error)
	State() (SpanState, error)
	TraceID() (*string, error)
	Tags() (map[string]interface{}, error)
}

type EventSource interface {
	Name() string
	ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error)
	Event(r *http.Request, payload []byte) (Event, error)
	Tracer() opentracing.Tracer
}
