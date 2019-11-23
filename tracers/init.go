package tracers

import (
	"context"
	"fmt"
	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/prometheus/common/log"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/urfave/cli"
	"io"
)

type LightstepCloser struct {
	tracer lightstep.Tracer
	ctx    context.Context
}

func (l LightstepCloser) Close() error {
	l.tracer.Close(l.ctx)
	return nil
}

func NewLightstepCloser(ctx context.Context, tracer lightstep.Tracer) LightstepCloser {
	return LightstepCloser{
		ctx:    ctx,
		tracer: tracer,
	}
}

// initJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func InitJaeger(ctx context.Context, service string) (opentracing.Tracer, io.Closer, error) {
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		return nil, nil, err
	}

	tracer, closer, err := cfg.New(service, jaegercfg.Logger(jaeger.StdLogger))
	if err != nil {
		return nil, nil, fmt.Errorf("ERROR: cannot init Jaeger: %v\n", err)
	}
	return tracer, closer, nil
}

func InitLightstep(service string, accessToken string) lightstep.Tracer {
	lightStepTracer := lightstep.NewTracer(lightstep.Options{
		Collector:   lightstep.Endpoint{},
		AccessToken: accessToken,
		Tags: map[string]interface{}{
			lightstep.ComponentNameKey: service,
		},
	})
	return lightStepTracer
}

type Initializer func(ctx context.Context, name string) (opentracing.Tracer, io.Closer, error)

func InitializerFromCLI(c *cli.Context, tracerName string) Initializer {
	log.Infof("building tracer initializer for: %q", tracerName)
	switch tracerName {
	case "jaeger":
		return InitJaeger
	case "mock":
		globalTracer := mocktracer.New()
		return func(context.Context, string) (opentracing.Tracer, io.Closer, error) {
			return globalTracer, NoopCloser{}, nil
		}
	case "lightstep":
		return func(ctx context.Context, service string) (opentracing.Tracer, io.Closer, error) {
			tracer := InitLightstep(
				service,
				c.String("tracer-access-token"),
			)
			return tracer, NewLightstepCloser(ctx, tracer), nil
		}
	default:
		return func(context.Context, string) (opentracing.Tracer, io.Closer, error) {
			return LoggingTracer{}, NoopCloser{}, nil
		}
	}

}
