package gitlab

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/opentracing/opentracing-go"
	"github.com/urfave/cli"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"net/http"
	"reflect"
)

const (
	sourceName string = "gitlab"
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

func (s *Source) SecretKey() []byte {
	return nil
}

func (s *Source) ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	if secretKey == nil {
		return ioutil.ReadAll(r.Body)
	}

	return nil, fmt.Errorf("does not support signing right now")
}

func (s *Source) Event(r *http.Request, payload []byte) (eventsources.Event, error) {
	var err error
	event, err := gitlab.ParseWebhook(gitlab.WebhookEventType(r), payload)
	if err != nil {
		return nil, err
	}

	switch event := event.(type) {
	case *gitlab.IssueEvent:
		return IssueEvent{event}, nil
	case *gitlab.MergeEvent:
		return MergeEvent{event}, nil
	case *gitlab.PipelineEvent:
		return PipelineEvent{event}, nil
	case *gitlab.JobEvent:
		return JobEvent{event}, nil
	default:
		err = fmt.Errorf("event type not supported, %+v", reflect.TypeOf(event))
	}
	return nil, err
}

func NewSource(tracer opentracing.Tracer) (eventsources.EventSource, error) {
	return &Source{
		tracer: tracer,
	}, nil
}

func NewFromCLI(c *cli.Context, tracer opentracing.Tracer) (eventsources.EventSource, error) {
	return NewSource(tracer)
}
