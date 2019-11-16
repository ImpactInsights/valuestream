package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/opentracing/opentracing-go"
	"io/ioutil"
	"net/http"
)

type EventSource struct {
	tracer opentracing.Tracer
}

func (es *EventSource) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, fmt.Errorf("unable to read body")
	}

	if secretKey != nil {
		sig := r.Header.Get(webhooks.SignatureHeader)
		mac := hmac.New(sha256.New, []byte(secretKey))
		mac.Write(body)
		expectedMAC := mac.Sum(nil)
		if !hmac.Equal([]byte(sig), expectedMAC) {
			return nil, fmt.Errorf("invalid event signature")
		}
	}

	return body, nil
}

func (es *EventSource) Event(r *http.Request, payload []byte) (eventsources.Event, error) {
	var e Event
	err := json.Unmarshal(payload, &e)
	return e, err
}

func (es *EventSource) Tracer() opentracing.Tracer {
	return es.tracer
}

func (es *EventSource) Name() string {
	return "custom_http"
}

func NewSource(tracer opentracing.Tracer) (*EventSource, error) {
	return &EventSource{
		tracer: tracer,
	}, nil
}
