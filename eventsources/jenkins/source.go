package jenkins

import (
	"encoding/json"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/opentracing/opentracing-go"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
)

const (
	sourceName string = "jenkins"
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

func (s *Source) Event(r *http.Request, payload []byte) (eventsources.Event, error) {
	var be BuildEvent
	err := json.Unmarshal(payload, &be)
	return be, err
}

func (s *Source) SecretKey() []byte {
	return nil
}

func (s *Source) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	return ioutil.ReadAll(r.Body)
}

func NewSource(tracer opentracing.Tracer) (*Source, error) {
	return &Source{
		tracer: tracer,
	}, nil
}

func NewFromCLI(c *cli.Context, tracer opentracing.Tracer) (eventsources.EventSource, error) {
	return NewSource(tracer)
}
