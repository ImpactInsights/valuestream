package tracers

import (
	"fmt"
	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io"
)

// initJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func InitJaeger(service string) (opentracing.Tracer, io.Closer) {
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		panic(err)
	}

	tracer, closer, err := cfg.New(service, jaegercfg.Logger(jaeger.StdLogger))
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return tracer, closer
}

func InitLightstep(service string, accessToken string) opentracing.Tracer {
	lightStepTracer := lightstep.NewTracer(lightstep.Options{
		Collector:   lightstep.Endpoint{},
		AccessToken: accessToken,
		Tags: map[string]interface{}{
			lightstep.ComponentNameKey: service,
		},
	})
	return lightStepTracer
}
