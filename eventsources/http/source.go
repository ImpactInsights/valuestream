package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
)

const (
	sourceName string = "customhttp"
)

type Source struct {
	tracer opentracing.Tracer
}

func (es *Source) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
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

func (es *Source) Event(r *http.Request, payload []byte) (eventsources.Event, error) {
	var e Event
	log.Debugf("raw event: %q", string(payload))
	err := json.Unmarshal(payload, &e)
	return e, err
}

func (es *Source) SecretKey() []byte {
	return nil
}

func (es *Source) Tracer() opentracing.Tracer {
	return es.tracer
}

func (es *Source) Name() string {
	return "custom_http"
}

func NewSource(tracer opentracing.Tracer) (eventsources.EventSource, error) {
	return &Source{
		tracer: tracer,
	}, nil
}

func NewFromCLI(c *cli.Context, tracer opentracing.Tracer) (eventsources.EventSource, error) {
	return NewSource(tracer)
}
