package webhooks

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
)

const (
	SignatureHeader = "X-VS-Signature"
)

type Tracers interface {
	RequestScoped(r *http.Request, es eventsources.EventSource) (opentracing.Tracer, io.Closer, error)
}

func New(
	es eventsources.EventSource,
	tracers Tracers,
	sk []byte,
	spans traces.SpanStore,
) (*Webhook, error) {

	return &Webhook{
		EventSource: es,
		Tracers:     tracers,
		SecretKey:   sk,
		Spans:       spans,
	}, nil
}

type Webhook struct {
	EventSource eventsources.EventSource
	Tracers     Tracers
	SecretKey   []byte
	Spans       traces.SpanStore
}

// secretKey inspects the request for a contexted define key
// and then falls back to a webhook instance defined key.
func (wh Webhook) secretKey(r *http.Request) []byte {
	sk := wh.SecretKey
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

	if err := wh.Spans.Set(ctx, spanID, traces.StoreEntry{Span: span}); err != nil {
		return err
	}

	return nil
}

func (wh *Webhook) handleEndEvent(ctx context.Context, tracer opentracing.Tracer, e eventsources.Event) error {
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

	// TODO add tags on end event
	isE, err := e.IsError()
	if err != nil {
		return err
	}

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
