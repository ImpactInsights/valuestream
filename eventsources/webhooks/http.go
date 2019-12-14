package webhooks

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"io"
	"net/http"
	"strconv"
)

const (
	SignatureHeader = "X-VS-Signature"
)

var (
	eventSource, _ = tag.NewKey("event_source")
	eventType, _   = tag.NewKey("event_type")
	eventErr, _    = tag.NewKey("error")

	EventStartCount = stats.Int64(
		"webhooks/event/start/count",
		"Number of events started",
		stats.UnitDimensionless,
	)

	EventStartCountView = &view.View{
		Name:        "webhooks/event/start/count",
		Description: "Number of events started",
		TagKeys:     []tag.Key{eventSource, eventType},
		Measure:     EventStartCount,
		Aggregation: view.Count(),
	}

	// Counts all end events
	// Timings may not be available due to what some EventSources expose through
	// their APIs, and/or start event may not be present in the span store.
	// Because of this keep track of all event end requests.
	EventEndCount = stats.Int64(
		"webhooks/event/end/count",
		"Number of events ended",
		stats.UnitDimensionless,
	)

	EventEndCountView = &view.View{
		Name:        "webhooks/event/end/count",
		Description: "Number of events started",
		TagKeys:     []tag.Key{eventSource, eventType, eventErr},
		Measure:     EventEndCount,
		Aggregation: view.Count(),
	}

	EventLatencyMs = stats.Float64(
		"webhooks/event/duration",
		"The latency in milliseconds",
		"ms",
	)

	EventLatencyView = &view.View{
		Name:        "webhooks/event/duration",
		Description: "Duration of events",
		TagKeys:     []tag.Key{eventSource, eventType, eventErr},
		Measure:     EventLatencyMs,
		Aggregation: view.Distribution(
			0,
			1.8e+6,   // 30 minutes
			3.6e+6,   // 1 hour
			1.08e+7,  // 3 hours
			2.16e+7,  // 6 hours
			4.32e+7,  // 12 hours
			8.64e+7,  // 24 hours
			2.592e+8, // 3 days
			6.048e+8, // 7 days
			1.21e+9,  // 2 weeks
			1.814e+9, // 3 weeks
			2.628e+9, // 1 month
		),
	}
)

type Tracers interface {
	RequestScoped(r *http.Request, es eventsources.EventSource) (opentracing.Tracer, io.Closer, error)
}

func New(
	es eventsources.EventSource,
	tracers Tracers,
	spans traces.SpanStore,
) (*Webhook, error) {

	return &Webhook{
		EventSource: es,
		Tracers:     tracers,
		Spans:       spans,
	}, nil
}

type Webhook struct {
	EventSource eventsources.EventSource
	Tracers     Tracers
	Spans       traces.SpanStore
}

// secretKey inspects the request for a contexted define key
// and then falls back to a webhook instance defined key.
func (wh Webhook) secretKey(r *http.Request) []byte {
	sk := wh.EventSource.SecretKey()
	k := r.Context().Value(CtxSecretTokenKey)
	v, ok := k.([]byte)
	if ok && v != nil {
		sk = v
	}
	return sk
}

func (wh *Webhook) Handler(w http.ResponseWriter, r *http.Request) {
	var payload []byte
	var err error
	var e eventsources.Event

	secretKey := wh.secretKey(r)

	if payload, err = wh.EventSource.ValidatePayload(r, secretKey); err != nil {
		log.WithFields(log.Fields{
			"error":   err.Error(),
			"payload": payload,
		}).Errorf("unable to validate request")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	if e, err = wh.EventSource.Event(r, payload); err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
			"event": e,
		}).Errorf("unable to convert payload to event")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	tracer, closer, err := wh.Tracers.RequestScoped(r, wh.EventSource)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
			"event": e,
		}).Errorf("error getting tracer from request")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}
	defer closer.Close()

	if err := wh.handleEvent(r.Context(), tracer, e); err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
			"event": e,
		}).Errorf("error processinng event")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	w.Write([]byte("success"))
}

func (wh *Webhook) handleStartEvent(ctx context.Context, tracer opentracing.Tracer, e eventsources.Event) error {
	ctx, err := tag.New(ctx,
		tag.Insert(eventSource, wh.EventSource.Name()),
		tag.Insert(eventType, e.OperationName()),
	)
	if err != nil {
		return err
	}

	stats.Record(ctx, EventStartCount.M(1))

	// check to see if this event has a parent span
	parentID, err := e.ParentSpanID()
	if err != nil {
		return err
	}

	opts := make([]opentracing.StartSpanOption, 0)

	// if it does than make sure to establish the ChildOf relationship
	if parentID != nil {
		entry, err := wh.Spans.Get(ctx, tracer, *parentID)
		if err != nil {
			return err
		}

		if entry != nil {
			opts = append(opts, opentracing.ChildOf(entry.Span.Context()))
		}
	}

	// Actually start the span
	span := tracer.StartSpan(
		e.OperationName(),
		opts...,
	)

	// Tag the span with all information present
	tags, err := e.Tags()
	if err != nil {
		return err
	}

	for k, v := range tags {
		span.SetTag(k, v)
	}

	// else we need to just set the span for future events
	spanID, err := e.SpanID()
	if err != nil {
		return err
	}

	if err := wh.Spans.Set(ctx, spanID, traces.NewStoreEntryFromSpan(span)); err != nil {
		return err
	}

	return nil
}

func (wh *Webhook) handleEndEvent(ctx context.Context, tracer opentracing.Tracer, e eventsources.Event) error {
	isE, err := e.IsError()
	if err != nil {
		return err
	}

	ctx, err = tag.New(ctx,
		tag.Insert(eventSource, wh.EventSource.Name()),
		tag.Insert(eventType, e.OperationName()),
		tag.Insert(eventErr, strconv.FormatBool(isE)),
	)
	if err != nil {
		return err
	}

	stats.Record(ctx, EventEndCount.M(1))

	spanID, err := e.SpanID()
	if err != nil {
		return err
	}

	entry, err := wh.Spans.Get(ctx, tracer, spanID)
	if err != nil {
		return err
	}

	if entry == nil {
		return traces.SpanMissingError{
			Err: fmt.Errorf("span not found for SpanID: %q", spanID),
		}
	}

	// If there's timing on the event than this should be treated as the
	// "source-of-truth" since it comes from the event source
	if timings, err := e.Timings(); err != nil && timings.Duration != nil {
		stats.Record(ctx, EventLatencyMs.M(float64(timings.Duration.Nanoseconds()/1e6)))
	} else {
		// there's no timing, we're unable to parse the timing or .... ?
		// use the time that ValueStream stored entry timing:
		stats.Record(ctx, EventLatencyMs.M(float64(entry.Duration().Nanoseconds()/1e6)))
	}

	// TODO add tags on end event
	entry.Span.SetTag("error", isE)
	entry.Span.Finish()

	if err := wh.Spans.Delete(ctx, spanID); err != nil {
		return err
	}

	return nil
}

func (wh *Webhook) handleEvent(ctx context.Context, tracer opentracing.Tracer, e eventsources.Event) error {
	// check to see if there are any current events for this event
	spanID, err := e.SpanID()
	if err != nil {
		return err
	}

	entry, err := wh.Spans.Get(ctx, tracer, spanID)
	if err != nil {
		return err
	}

	var prevState *eventsources.EventState
	if entry != nil {
		prevState = entry.State
	}

	state, err := e.State(prevState)

	log.WithFields(log.Fields{
		"state": state,
		"error": err,
	}).Debug("webhooks.http.handleEvent event state")

	if err != nil {
		return err
	}

	switch state {
	case eventsources.StartState:
		return wh.handleStartEvent(ctx, tracer, e)
	case eventsources.EndState:
		return wh.handleEndEvent(ctx, tracer, e)
	case eventsources.TransitionState:
		// handleTransitionEvent(ctx, tracer, e)
		if err := wh.handleEndEvent(ctx, tracer, e); err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Errorf("webhooks.handleEvent")
		}
		return wh.handleStartEvent(ctx, tracer, e)
	}

	return nil
}
