package http

import (
	"github.com/ImpactInsights/valuestream/eventsources"
	"strings"
)

type Event struct {
	Identifier string `json:"id"`
	Action     string
	ParentID   *string
	Error      bool
	Namespace  string
	Type       string
	Metadata   map[string]interface{}
}

func (e Event) Timings() eventsources.EventTimings { return eventsources.EventTimings{} }

// ID is how an INDIVIDUAL event is referenced internally
func (e Event) SpanID() (string, error) {

	return strings.Join([]string{
		"vstrace",
		sourceName,
		e.Type,
		e.Namespace,
		e.Identifier,
	}, "-"), nil
}

func (e Event) OperationName() string {
	return e.Type
}

// ParentSpanID allows this HTTP event to reference any other
// event.
func (e Event) ParentSpanID() (*string, error) {
	return e.ParentID, nil
}

func (e Event) IsError() (bool, error) {
	return e.Error, nil
}

func (e Event) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
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

	tags[eventsources.SourceNameTag] = sourceName

	sID, _ := e.SpanID()
	tags[eventsources.SpanIDTag] = sID

	if pID, _ := e.ParentSpanID(); pID != nil {
		tags[eventsources.ParentSpanIDTag] = *pID
	}

	return tags, nil
}
