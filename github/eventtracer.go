package github

import (
	"context"
	"fmt"
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
	case traces.StartState:
		span := et.Tracer.StartSpan(
			"issue",
		)
		for k, v := range ie.Tags() {
			span.SetTag(k, v)
		}

		if tID, found := ie.TraceID(); found {
			et.traces.Set(ctx, tID, span)
		}

	case traces.EndState:
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

		span, ok := et.traces.Get(ctx, tID)
		if !ok {
			return traces.SpanMissingError{
				Err: fmt.Errorf("span not found for github span: %q", tID),
			}
		}
		span.Finish()
		// TODO add error/state result
		et.traces.Delete(ctx, tID)
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
	case traces.StartState:
		parentID, found := pre.ParentSpanID()
		opts := make([]opentracing.StartSpanOption, 0)

		if found {
			parentSpan, hasParent := et.traces.Get(ctx, parentID)
			if hasParent {
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
		et.spans.Set(ctx, pre.ID(), span)
		if ref := pre.BranchRef(); ref != nil {
			// TODO This will need to be namespaced for next repo
			et.traces.Set(ctx, *ref, span)
		} else {
			log.Warnf("no branch")
		}
		return nil

	case traces.EndState:
		sID := pre.ID()
		span, ok := et.spans.Get(ctx, sID)
		if !ok {
			return traces.SpanMissingError{
				Err: fmt.Errorf("span not found for github span: %q", sID),
			}
		}
		span.Finish()
		// TODO add error/state result

		et.spans.Delete(ctx, pre.ID())
		if ref := pre.BranchRef(); ref != nil {
			et.traces.Delete(ctx, *ref)
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
