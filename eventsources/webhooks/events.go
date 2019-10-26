package webhooks

import "github.com/ImpactInsights/valuestream/events"

type Event interface {
	SpanID() string
	ParentSpanID() string
	IsError() bool
	State() events.SpanState
}
