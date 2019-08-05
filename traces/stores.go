package traces

import (
	"context"
	"fmt"
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

type SpanState int

const (
	StartState SpanState = iota
	EndState
	IntermediaryState
)

type SpanStore interface {
	Get(id string) (opentracing.Span, bool)
	Set(id string, span opentracing.Span)
	Delete(id string) bool
	Count() int
}

type Spans struct {
	spans map[string]opentracing.Span
	mu    *sync.Mutex
}

func (s *Spans) Get(id string) (opentracing.Span, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	span, ok := s.spans[id]
	return span, ok
}

func (s *Spans) Set(id string, span opentracing.Span) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spans[id] = span
}

func (s *Spans) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.spans, id)
	return true
}

func (s *Spans) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.spans)
}

func NewMemoryUnboundedSpanStore() *Spans {
	return &Spans{
		spans: make(map[string]opentracing.Span),
		mu:    &sync.Mutex{},
	}
}

type BufferedSpans struct {
	spans      map[string]int
	mu         *sync.Mutex
	buf        []*idSpan
	totalSpans int
}

type idSpan struct {
	id   string
	span opentracing.Span
}

// Set calculates the index of the buffer to use and
// then sets the span in the buffer and updates the map
// to associate the id with the index
func (s *BufferedSpans) Set(id string, span opentracing.Span) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalSpans++
	i := s.totalSpans % len(s.buf)
	if curr := s.buf[i]; curr != nil {
		delete(s.spans, curr.id)
	}

	s.buf[i] = &idSpan{
		id:   id,
		span: span,
	}

	// remove the map entry as well or else it can grow unbounded
	s.spans[id] = i
}

// TODO delete may be missing now that we have a circular buffer
func (s *BufferedSpans) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	i, ok := s.spans[id]
	if !ok {
		return false
	}
	s.buf[i] = nil
	delete(s.spans, id)
	return true
}

func (s *BufferedSpans) Get(id string) (opentracing.Span, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.spans[id]
	if !ok {
		return nil, false
	}
	return s.buf[i].span, true
}

func (s *BufferedSpans) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.spans)
}

func (s *BufferedSpans) Monitor(ctx context.Context, interval time.Duration, name string) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			bufSize := float64(len(s.buf))
			currSize := float64(len(s.spans))
			s.mu.Unlock()
			percentage := currSize / bufSize

			log.WithFields(log.Fields{
				"buffer_size":       bufSize,
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

func NewBufferedSpanStore(numBufferedSpans int) (*BufferedSpans, error) {
	if numBufferedSpans <= 0 {
		return nil, fmt.Errorf("buffer size must be > 0, recieved: %d", numBufferedSpans)
	}

	s := &BufferedSpans{
		spans: make(map[string]int),
		mu:    &sync.Mutex{},
		buf:   make([]*idSpan, numBufferedSpans),
	}

	return s, nil
}
