package traces

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var (
	bufferedSpansTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "buffered_spans_total",
			Help: "Gauge total number of current buffered spans",
		},
		[]string{"buffer_name"},
	)
	bufferedSpansPercentage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "buffered_spans_percentage",
			Help: "Gauge percentage of buffer in use",
		},
		[]string{"buffer_name"},
	)
)

func init() {
	prometheus.MustRegister(
		bufferedSpansTotal,
		bufferedSpansPercentage,
	)
}

type StoreEntry struct {
	Span      opentracing.Span
	State     *eventsources.EventState
	CreatedAt time.Time
}

func NewStoreEntryFromSpan(span opentracing.Span) StoreEntry {
	return StoreEntry{
		Span:      span,
		CreatedAt: time.Now().UTC(),
	}
}

func (se StoreEntry) Duration() time.Duration {
	return time.Now().Sub(se.CreatedAt)
}

type SpanStore interface {
	Get(ctx context.Context, tracer opentracing.Tracer, id string) (*StoreEntry, error)
	Set(ctx context.Context, id string, entry StoreEntry) error
	Delete(ctx context.Context, id string) error
	Count() (int, error)
}

type Spans struct {
	spans map[string]StoreEntry
	mu    *sync.Mutex
}

func (s *Spans) Get(ctx context.Context, tracer opentracing.Tracer, id string) (*StoreEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.spans[id]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

func (s *Spans) Set(ctx context.Context, id string, entry StoreEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans[id] = entry
	return nil
}

func (s *Spans) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.spans, id)
	return nil
}

func (s *Spans) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.spans), nil
}

func NewMemoryUnboundedSpanStore() *Spans {
	return &Spans{
		spans: make(map[string]StoreEntry),
		mu:    &sync.Mutex{},
	}
}

// BufferedSpans only allows a fixed number of spans at any one time.
// If the max allowed has been reached it will reject new spans.
type BufferedSpans struct {
	spans           map[string]StoreEntry
	mu              *sync.Mutex
	maxAllowedSpans int
}

type idSpan struct {
	id   string
	span opentracing.Span
}

// Set checks to see if there is a free space in the map
// if there is then it inserts, if there is no free space
// it returns an error.
func (s *BufferedSpans) Set(ctx context.Context, id string, entry StoreEntry) error {
	log.Debugf("BufferedSpans.Set(), id: %q", id)
	s.mu.Lock()
	defer s.mu.Unlock()

	// increment the total amount of spans that we've seen
	if len(s.spans) == s.maxAllowedSpans {
		return fmt.Errorf("maxAllowedSpans: %d reached", s.maxAllowedSpans)
	}

	s.spans[id] = entry
	return nil
}

// Delete removes the id, if present, from the buffered spans collection.
func (s *BufferedSpans) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.spans, id)
	return nil
}

func (s *BufferedSpans) Get(ctx context.Context, tracer opentracing.Tracer, id string) (*StoreEntry, error) {
	log.Debugf("BufferedSpans.Get(), id: %q", id)
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.spans[id]
	if !ok {
		return nil, nil
	}
	return &i, nil
}

func (s *BufferedSpans) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.spans), nil
}

func (s *BufferedSpans) DeleteAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans = make(map[string]StoreEntry)
	return nil
}

func (s *BufferedSpans) Monitor(ctx context.Context, interval time.Duration, name string) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			currSize := float64(len(s.spans))
			s.mu.Unlock()
			percentage := currSize / float64(s.maxAllowedSpans)

			log.WithFields(log.Fields{
				"buffer_size":       s.maxAllowedSpans,
				"curr_size":         currSize,
				"buffer_percentage": percentage,
				"name":              name,
			}).Info("buffered_spans_state")

			bufferedSpansTotal.With(prometheus.Labels{"buffer_name": name}).Set(currSize)
			bufferedSpansPercentage.With(prometheus.Labels{"buffer_name": name}).Set(percentage)
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func NewBufferedSpanStore(maxAllowedSpans int) (*BufferedSpans, error) {
	if maxAllowedSpans <= 0 {
		return nil, fmt.Errorf("maxAllowedSpans must be > 0, received: %d", maxAllowedSpans)
	}

	s := &BufferedSpans{
		spans:           make(map[string]StoreEntry),
		mu:              &sync.Mutex{},
		maxAllowedSpans: maxAllowedSpans,
	}

	return s, nil
}
