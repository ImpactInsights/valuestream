package eventsources

import (
	"github.com/opentracing/opentracing-go"
	"net/http"
	"time"
)

type SpanState string

type EventState string

const (
	TracePrefix string = "vstrace"
)

const (
	StartState        SpanState = "start"
	EndState          SpanState = "end"
	IntermediaryState SpanState = "intermediary"
	TransitionState   SpanState = "transition"
	UnknownState      SpanState = "unknown"
)

type EventTimings interface {
	StartTime() time.Time
	EndTime() time.Time
	Duration() time.Duration
}

type Event interface {
	SpanID() (string, error)
	OperationName() string
	ParentSpanID() (*string, error)
	IsError() (bool, error)
	State(prev *EventState) (SpanState, error)
	Tags() (map[string]interface{}, error)
	Timings() EventTimings
}

type EventSource interface {
	Name() string
	ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error)
	Event(r *http.Request, payload []byte) (Event, error)
	Tracer() opentracing.Tracer
	SecretKey() []byte
}
