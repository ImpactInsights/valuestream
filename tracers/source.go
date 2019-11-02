package tracers

import (
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/opentracing/opentracing-go"
	"io"
	"net/http"
)

type NoopCloser struct{}

func (nc NoopCloser) Close() error {
	return nil
}

// Source tracer delegates to the EventSource provided tracer
type Source struct{}

func (s Source) RequestScoped(r *http.Request, es eventsources.EventSource) (opentracing.Tracer, io.Closer, error) {
	return es.Tracer(), NoopCloser{}, nil
}

func NewRequestScopedUsingSources() Source {
	return Source{}
}
