package github

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/google/go-github/github"
	"github.com/opentracing/opentracing-go"
	"io/ioutil"
	"net/http"
)

type Source struct {
	tracer opentracing.Tracer
}

func (s Source) Name() string {
	return "github"
}

func (s *Source) Tracer() opentracing.Tracer {
	return s.tracer
}

func (s *Source) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	if secretKey == nil {
		return ioutil.ReadAll(r.Body)
	}

	return github.ValidatePayload(r, secretKey)
}

func (s *Source) Event(r *http.Request, payload []byte) (webhooks.Event, error) {
	var err error
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return nil, err
	}

	switch event := event.(type) {
	case *github.IssuesEvent:
		return IssuesEvent{event}, nil
	case *github.PullRequestEvent:
		return PREvent{event}, nil
	default:
		err = fmt.Errorf("event type not supported, %+v", event)
	}
	return nil, err
}

func NewSource(tracer opentracing.Tracer) (*Source, error) {
	return &Source{
		tracer: tracer,
	}, nil
}
