package jenkins

import (
	"context"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
)

// EVENT STRUCTURES HERE
// https://github.com/jenkinsci/statistics-gatherer-plugin

type EventTracer struct {
	spans traces.SpanStore

	Tracer opentracing.Tracer
	traces traces.SpanStore
}

func (et *EventTracer) handleBuild(ctx context.Context, be *BuildEvent) error {
	switch be.State() {
	case startState:
		parentID, found := be.ParentSpanID()

		log.WithFields(log.Fields{
			"trace_id":  parentID,
			"parent_id": parentID,
			"found":     found,
			"tags":      be.Tags(),
		}).Info("jenkins_start_build")

		opts := make([]opentracing.StartSpanOption, 0)

		if found {
			parentSpan, err := et.traces.Get(ctx, et.Tracer, parentID)
			if err != nil {
				return err
			}
			if parentSpan != nil {
				opts = append(opts, opentracing.ChildOf(parentSpan.Context()))
			}
		}

		span := et.Tracer.StartSpan(
			be.OperationName(),
			opts...,
		)
		for k, v := range be.Tags() {
			span.SetTag(k, v)
		}
		return et.spans.Set(ctx, be.ID(), span)

	case endState:
		span, err := et.spans.Get(ctx, et.Tracer, be.ID())
		if err != nil {
			return err
		}

		if span == nil {
			log.WithFields(log.Fields{
				"service": "jenkins",
				"event":   "build",
				"span_id": be.ID(),
			}).Warn("no span found")
			return nil
		}
		for k, v := range be.Tags() {
			span.SetTag(k, v)
		}
		isErr := be.Result != "SUCCESS"
		span.SetTag("error", isErr)
		span.Finish()
		return et.spans.Delete(ctx, be.ID())
	}

	return nil
}

func NewEventTracer(tracer opentracing.Tracer, ts traces.SpanStore, spans traces.SpanStore) *EventTracer {
	return &EventTracer{
		Tracer: tracer,
		spans:  spans,
		traces: ts,
	}
}
