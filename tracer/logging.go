package tracer

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	logger "github.com/sirupsen/logrus"
)

type LoggingTracer struct{}

type loggingSpan struct {
	operationName string
	opts          []opentracing.StartSpanOption
}
type loggingSpanContext struct{}

var (
	defaultNoopSpanContext = loggingSpanContext{}
	defaultNoopSpan        = loggingSpan{}
	defaultLoggingTracer   = LoggingTracer{}
)

const (
	emptyString = ""
)

// loggingSpanContext:
func (n loggingSpanContext) ForeachBaggageItem(handler func(k, v string) bool) {}

// loggingSpan:
func (n loggingSpan) Context() opentracing.SpanContext                { return defaultNoopSpanContext }
func (n loggingSpan) SetBaggageItem(key, val string) opentracing.Span { return defaultNoopSpan }
func (n loggingSpan) BaggageItem(key string) string                   { return emptyString }
func (n loggingSpan) SetTag(key string, value interface{}) opentracing.Span {
	logger.WithFields(logger.Fields{
		"operation_name": n.operationName,
		"opts":           n.opts,
		"key":            key,
		"value":          value,
	}).Info("span.SetTag()")
	return n
}
func (n loggingSpan) LogFields(fields ...log.Field) {}
func (n loggingSpan) LogKV(keyVals ...interface{})  {}
func (n loggingSpan) Finish() {
	logger.WithFields(logger.Fields{
		"operation_name": n.operationName,
		"opts":           n.opts,
	}).Infof("span.Finish()")
}
func (n loggingSpan) FinishWithOptions(opts opentracing.FinishOptions)       {}
func (n loggingSpan) SetOperationName(operationName string) opentracing.Span { return n }
func (n loggingSpan) Tracer() opentracing.Tracer                             { return defaultLoggingTracer }
func (n loggingSpan) LogEvent(event string)                                  {}
func (n loggingSpan) LogEventWithPayload(event string, payload interface{})  {}
func (n loggingSpan) Log(data opentracing.LogData)                           {}

// StartSpan belongs to the Tracer interface.
func (n LoggingTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	logger.WithFields(logger.Fields{
		"operation_name": operationName,
		"opts":           opts,
	}).Infof("tracer.StartSpan()")
	return loggingSpan{
		operationName: operationName,
		opts:          opts,
	}
}

// Inject belongs to the Tracer interface.
func (n LoggingTracer) Inject(sp opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return nil
}

// Extract belongs to the Tracer interface.
func (n LoggingTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return nil, opentracing.ErrSpanContextNotFound
}
