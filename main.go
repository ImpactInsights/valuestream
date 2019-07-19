package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/ImpactInsights/valuestream/github"
	"github.com/ImpactInsights/valuestream/jenkins"
	"github.com/ImpactInsights/valuestream/tracer"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/gorilla/mux"
	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const envLogLevel string = "VS_LOG_LEVEL"

var (
	counter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	// duration is partitioned by the HTTP method and handler. It uses custom
	// buckets based on the expected request duration.
	duration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		},
		[]string{"handler", "method"},
	)

	responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{},
	)
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	switch os.Getenv(envLogLevel) {
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "WARN":
		log.SetLevel(log.WarnLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}

	prometheus.MustRegister(
		duration,
		counter,
	)
}

// initJaeger returns an instance of Jaeger Tracer that samples 100% of traces and logs all spans to stdout.
func initJaeger(service string) (opentracing.Tracer, io.Closer) {
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

func initLightstep(service string, accessToken string) opentracing.Tracer {
	lightStepTracer := lightstep.NewTracer(lightstep.Options{
		Collector:   lightstep.Endpoint{},
		AccessToken: accessToken,
		Tags: map[string]interface{}{
			lightstep.ComponentNameKey: service,
		},
	})
	return lightStepTracer
}

func main() {
	var addr = flag.String("addr", "127.0.0.1:5000", "addr/port for the tes")
	var tracerImplName = flag.String("tracer", "logging", "tracer implementation to use: 'logger|jaeger|lightstep'")
	flag.Parse()

	ctx := context.Background()
	var githubTracer opentracing.Tracer
	var jenkinsTracer opentracing.Tracer

	switch *tracerImplName {
	case "jaeger":
		var githubTracerCloser io.Closer
		var jenkinsTracerCloser io.Closer

		log.Infof("initializing tracer: jaeger")
		githubTracer, githubTracerCloser = initJaeger("github")
		defer githubTracerCloser.Close()

		jenkinsTracer, jenkinsTracerCloser = initJaeger("jenkins")
		defer jenkinsTracerCloser.Close()

	case "lightstep":

		log.Infof("initializing tracer: lightstep")

		accessToken := os.Getenv("VS_LIGHTSTEP_ACCESS_TOKEN")
		githubTracer = initLightstep("github", accessToken)
		jenkinsTracer = initLightstep("jenkins", accessToken)

		defer lightstep.Close(ctx, githubTracer)
		defer lightstep.Close(ctx, jenkinsTracer)

	default:
		log.Infof("initializing tracer: logging")
		githubTracer = tracer.LoggingTracer{}
		jenkinsTracer = tracer.LoggingTracer{}
	}

	ts, err := traces.NewBufferedSpanCache(1000)
	if err != nil {
		panic(err)
	}
	go ts.Monitor(ctx, time.Second*60, "traces")

	jenkinsSpans, err := traces.NewBufferedSpanCache(500)
	if err != nil {
		panic(err)
	}
	go jenkinsSpans.Monitor(ctx, time.Second*60, "jenkins")

	jenkins := jenkins.NewWebhook(
		jenkins.NewEventTracer(
			jenkinsTracer,
			ts,
			jenkinsSpans,
		),
	)

	githubSpans, err := traces.NewBufferedSpanCache(500)
	if err != nil {
		panic(err)
	}
	go githubSpans.Monitor(ctx, time.Second*20, "github")

	var githubSecretToken []byte

	if val, ok := os.LookupEnv("GITHUB_WEBHOOK_SECRET_TOKEN"); ok {
		githubSecretToken = []byte(val)
	}

	github := github.NewWebhook(
		github.NewEventTracer(
			githubTracer,
			ts,
			githubSpans,
		),
		githubSecretToken,
	)

	r := mux.NewRouter()

	r.Handle("/jenkins/",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "jenkins_build"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(jenkins.HTTPBuildHandler),
				),
			),
		),
	)

	r.Handle("/github/",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "github"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(github.Handler),
				),
			),
		),
	)
	r.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Handler:      r,
		Addr:         *addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Infof("Starting Server: %q", *addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	waitForShutdown(srv)
}

func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting down")
	os.Exit(0)
}
