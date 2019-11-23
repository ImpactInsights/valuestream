package jiracloud

import (
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/opentracing/opentracing-go"
	"io/ioutil"
	"net/http"
)

const (
	sourceName string = "jiracloud"
)

type Source struct {
	tracer opentracing.Tracer
}

func (s Source) Name() string {
	return sourceName
}

func (s *Source) Tracer() opentracing.Tracer {
	return s.tracer
}

func (s *Source) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	if secretKey == nil {
		return ioutil.ReadAll(r.Body)
	}

	return nil, fmt.Errorf("does not support signing right now")
}

func (s *Source) Event(r *http.Request, payload []byte) (eventsources.Event, error) {
	var e Event
	fmt.Println(string(payload))
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}

	// get the specific event type from the wrapper even
	switch e.Type() {
	case sprintEvent:
		var se SprintEvent
		err := json.Unmarshal(payload, &se)
		return se, err
	case issueEvent:
		var ie IssueEvent
		err := json.Unmarshal(payload, &ie)
		return ie, err
	}

	return nil, fmt.Errorf("event type: %q, not supported", e.WebhookEvent)
}

func NewSource(tracer opentracing.Tracer) (*Source, error) {
	return &Source{
		tracer: tracer,
	}, nil
}
