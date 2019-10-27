package http

import (
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
)

type Event struct {
	Identifier string `json:"id"`
	Action     string
	ParentID   *string
	Metadata   map[string]interface{}
}

// ID is how an INDIVIDUAL event is referenced internally
func (e Event) ID() string {
	return e.Identifier
}

// ParentSpanID allows this HTTP event to reference any other
// event.
func (e Event) ParentSpanID() *string {
	return e.ParentID
}

// TraceID is how other events reference this event!
func (e Event) TraceID() (string, bool) {
	return "", false
}

func (e Event) State() webhooks.SpanState {
	switch e.Action {
	case "start":
		return webhooks.StartState
	case "end":
		return webhooks.EndState
	}
	return webhooks.IntermediaryState
}

func (e Event) Tags() map[string]interface{} {

	tags := make(map[string]interface{})

	for k, v := range e.Metadata {
		tags[k] = v
	}

	tags["service"] = "http"
	return tags
}
