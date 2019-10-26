package github

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/events"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/google/go-github/github"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
)

type EventTracer struct {
	Tracer opentracing.Tracer
	spans  traces.SpanStore
	traces traces.SpanStore
}

func (et *EventTracer) handleIssue(ctx context.Context, issue *github.IssuesEvent) error {
	ie := IssuesEvent{issue}

	log.WithFields(log.Fields{
		"state": ie.State(),
		"tags":  ie.Tags(),
	}).Infof("handleIssue()")

	switch ie.State() {
	case events.StartState:
		span := et.Tracer.StartSpan(
			"issue",
		)
		for k, v := range ie.Tags() {
			span.SetTag(k, v)
		}

		if tID, found := ie.TraceID(); found {
			return et.traces.Set(ctx, tID, span)
		}
		return nil

	case events.EndState:
		tID, found := ie.TraceID()
		// this means the payload does not contain an identifiable trace
		// cannot continue
		if !found {
			return traces.SpanMissingIDError{
				// TODO add more context
				// enough to identify this event and make this actionable
				Err: fmt.Errorf("span missing id for github"),
			}
		}

		span, err := et.traces.Get(ctx, et.Tracer, tID)
		if err != nil {
			return err
		}
		if span == nil {
			return traces.SpanMissingError{
				Err: fmt.Errorf("span not found for github span: %q", tID),
			}
		}
		span.Finish()
		// TODO add error/state result
		return et.traces.Delete(ctx, tID)
	}

	return nil
}

func (et *EventTracer) handlePullRequest(ctx context.Context, pr *github.PullRequestEvent) error {
	pre := PREvent{pr}

	log.WithFields(log.Fields{
		"state": pre.State(),
		"tags":  pre.Tags(),
	}).Infof("handlePullRequest()")

	switch pre.State() {
	case events.StartState:
		parentID, found := pre.ParentSpanID()
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
			"pull_request",
			opts...,
		)
		for k, v := range pre.Tags() {
			span.SetTag(k, v)
		}
		if err := et.spans.Set(ctx, pre.ID(), span); err != nil {
			return err
		}

		if ref := pre.BranchRef(); ref != nil {
			// TODO This will need to be namespaced for next repo
			return et.traces.Set(ctx, *ref, span)
		} else {
			log.Warnf("no branch")
		}
		return nil

	case events.EndState:
		sID := pre.ID()
		span, err := et.spans.Get(ctx, et.Tracer, sID)
		if err != nil {
			return err
		}
		if span == nil {
			return traces.SpanMissingError{
				Err: fmt.Errorf("span not found for github span: %q", sID),
			}
		}
		span.Finish()
		// TODO add error/state result

		if err := et.spans.Delete(ctx, pre.ID()); err != nil {
			return err
		}

		if ref := pre.BranchRef(); ref != nil {
			return et.traces.Delete(ctx, *ref)
		}
	}
	// check for start/end
	// if start check to see if Traces has an event present
	return nil
}

func (et *EventTracer) handleEvent(ctx context.Context, event interface{}) error {
	var err error
	switch event := event.(type) {
	case *github.IssuesEvent:
		err = et.handleIssue(ctx, event)
	case *github.PullRequestEvent:
		err = et.handlePullRequest(ctx, event)
	default:
		err = fmt.Errorf("event type not supported, %+v", event)
	}
	return err
}

func NewEventTracer(tracer opentracing.Tracer, ts traces.SpanStore, spans traces.SpanStore) *EventTracer {
	return &EventTracer{
		Tracer: tracer,
		spans:  spans,
		traces: ts,
	}
}
