package eventsources

import (
	"encoding/json"
	"github.com/opentracing/opentracing-go"
	"io/ioutil"
	"net/http"
	"os"
)

type StubEventSource struct {
	ValidatePayloadFn func(r *http.Request, secretKey []byte) ([]byte, error)
	NameReturn        string
	EventFn           func(*http.Request, []byte) (Event, error)
	TracerReturn      opentracing.Tracer
}

func (s StubEventSource) Name() string {
	return s.NameReturn
}

func (s StubEventSource) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	return s.ValidatePayloadFn(r, secretKey)
}

func (s StubEventSource) Event(r *http.Request, payload []byte) (Event, error) {
	return s.EventFn(r, payload)
}

func (s StubEventSource) Tracer() opentracing.Tracer {
	return s.TracerReturn
}

type StubEvent struct {
	SpanIDReturn            string
	SpanIDReturnError       error
	OperationNameReturn     string
	ParentSpanIDReturn      *string
	ParentSpanIDReturnError error
	IsErrorReturn           bool
	IsErrorReturnError      error
	StateReturn             SpanState
	StateReturnError        error
	TraceIDReturn           *string
	TraceIDReturnError      error
	TagsReturn              map[string]interface{}
	TagsReturnError         error
}

func (s StubEvent) SpanID() (string, error) {
	return s.SpanIDReturn, s.SpanIDReturnError
}
func (s StubEvent) OperationName() string {
	return s.OperationNameReturn
}
func (s StubEvent) ParentSpanID() (*string, error) {
	return s.ParentSpanIDReturn, s.ParentSpanIDReturnError
}
func (s StubEvent) IsError() (bool, error) {
	return s.IsErrorReturn, s.IsErrorReturnError
}
func (s StubEvent) State(prev *EventState) (SpanState, error) {
	return s.StateReturn, s.StateReturnError
}
func (s StubEvent) TraceID() (*string, error) {
	return s.TraceIDReturn, s.TraceIDReturnError
}
func (s StubEvent) Tags() (map[string]interface{}, error) {
	return s.TagsReturn, s.TagsReturnError
}

type TestEvent struct {
	Headers map[string]string
	Payload interface{}
}

func NewTestEventFromFixturePath(path string) (*TestEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var te TestEvent

	if err = json.Unmarshal(bs, &te); err != nil {
		return nil, err
	}

	return &te, nil
}
