package http

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
)

type Event struct {
	Identifier string `json:"id"`
	Action     string
	ParentID   *string
	Error      bool
	Metadata   map[string]interface{}
}

// ID is how an INDIVIDUAL event is referenced internally
func (e Event) SpanID() (string, error) {
	return e.Identifier, nil
}

func (e Event) OperationName() string {
	return "custom_http"
}

// ParentSpanID allows this HTTP event to reference any other
// event.
func (e Event) ParentSpanID() (*string, error) {
	return e.ParentID, nil
}

func (e Event) IsError() (bool, error) {
	return e.Error, nil
}

// TraceID is how other events reference this event!
func (e Event) TraceID() (*string, error) {
	spanID, err := e.SpanID()
	if err != nil {
		return nil, err
	}

	traceID := fmt.Sprintf("vstrace-custom-http-%s", spanID)
	return &traceID, nil
}

func (e Event) State() (eventsources.SpanState, error) {
	switch e.Action {
	case "start":
		return eventsources.StartState, nil
	case "end":
		return eventsources.EndState, nil
	}
	return eventsources.IntermediaryState, nil
}

func (e Event) Tags() (map[string]interface{}, error) {

	tags := make(map[string]interface{})

	for k, v := range e.Metadata {
		tags[k] = v
	}

	tags["service"] = "http"
	return tags, nil
}
