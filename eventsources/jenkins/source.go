package jenkins

import (
	"encoding/json"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/opentracing/opentracing-go"
	"io/ioutil"
	"net/http"
)

type Source struct {
	tracer opentracing.Tracer
}

func (s Source) Name() string {
	return "jenkins"
}

func (s *Source) Tracer() opentracing.Tracer {
	return s.tracer
}

func (s *Source) Event(r *http.Request, payload []byte) (webhooks.Event, error) {
	var be BuildEvent
	err := json.Unmarshal(payload, &be)
	return be, err
}

func (s *Source) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	return ioutil.ReadAll(r.Body)
}