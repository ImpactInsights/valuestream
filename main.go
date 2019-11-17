package main

import (
	"context"
	"flag"
	"github.com/ImpactInsights/valuestream/eventsources/github"
	gitlab2 "github.com/ImpactInsights/valuestream/eventsources/gitlab"
	customhttp "github.com/ImpactInsights/valuestream/eventsources/http"
	"github.com/ImpactInsights/valuestream/eventsources/jenkins"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/ImpactInsights/valuestream/tracers"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
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

func main() {
	var addr = flag.String("addr", "127.0.0.1:5000", "addr/port for the tes")
	var tracerImplName = flag.String("tracer", "logging", "tracer implementation to use: 'logger|jaeger|lightstep'")
	flag.Parse()

	ctx := context.Background()
	var githubTracer opentracing.Tracer
	var jenkinsTracer opentracing.Tracer
	var customHTTPTracer opentracing.Tracer
	var gitlabTracer opentracing.Tracer

	switch *tracerImplName {
	case "jaeger":
		var githubTracerCloser io.Closer
		var jenkinsTracerCloser io.Closer
		var customHTTPTracerCloser io.Closer
		var gitlabTracerCloser io.Closer

		log.Infof("initializing tracer: jaeger")
		githubTracer, githubTracerCloser = tracers.InitJaeger("github")
		defer githubTracerCloser.Close()

		jenkinsTracer, jenkinsTracerCloser = tracers.InitJaeger("jenkins")
		defer jenkinsTracerCloser.Close()

		customHTTPTracer, customHTTPTracerCloser = tracers.InitJaeger("custom_http")
		defer customHTTPTracerCloser.Close()

		gitlabTracer, gitlabTracerCloser = tracers.InitJaeger("gitlab")
		defer gitlabTracerCloser.Close()

	case "lightstep":
		log.Infof("initializing tracer: lightstep")

		accessToken := os.Getenv("VS_LIGHTSTEP_ACCESS_TOKEN")
		githubTracer = tracers.InitLightstep("github", accessToken)
		jenkinsTracer = tracers.InitLightstep("jenkins", accessToken)
		customHTTPTracer = tracers.InitLightstep("custom_http", accessToken)
		gitlabTracer = tracers.InitLightstep("gitlab", accessToken)

		defer lightstep.Close(ctx, githubTracer)
		defer lightstep.Close(ctx, jenkinsTracer)
		defer lightstep.Close(ctx, customHTTPTracer)
		defer lightstep.Close(ctx, gitlabTracer)

	default:
		log.Infof("initializing tracer: logging")
		githubTracer = tracers.LoggingTracer{}
		jenkinsTracer = tracers.LoggingTracer{}
		customHTTPTracer = tracers.LoggingTracer{}
		gitlabTracer = tracers.LoggingTracer{}
	}

	spans, err := traces.NewBufferedSpanStore(1000)
	if err != nil {
		panic(err)
	}

	jenkinsSource, err := jenkins.NewSource(jenkinsTracer)
	if err != nil {
		panic(err)
	}

	jenkins, err := webhooks.New(
		jenkinsSource,
		tracers.NewRequestScopedUsingSources(),
		nil,
		spans,
	)
	if err != nil {
		panic(err)
	}

	var githubSecretToken []byte

	if val, ok := os.LookupEnv("GITHUB_WEBHOOK_SECRET_TOKEN"); ok {
		githubSecretToken = []byte(val)
	}

	githubSource, err := github.NewSource(githubTracer)
	if err != nil {
		panic(err)
	}

	github, err := webhooks.New(
		githubSource,
		tracers.NewRequestScopedUsingSources(),
		githubSecretToken,
		spans,
	)
	if err != nil {
		panic(err)
	}

	customHTTP, err := customhttp.NewSource(customHTTPTracer)
	if err != nil {
		panic(err)
	}

	customHTTPWebhook, err := webhooks.New(
		customHTTP,
		tracers.NewRequestScopedUsingSources(),
		nil,
		spans,
	)

	gitlab, err := gitlab2.NewSource(gitlabTracer)

	gitlabWebhook, err := webhooks.New(
		gitlab,
		tracers.NewRequestScopedUsingSources(),
		nil,
		spans,
	)

	r := mux.NewRouter()

	r.Handle("/jenkins",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "jenkins_build"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(jenkins.Handler),
				),
			),
		),
	)

	r.Handle("/customhttp",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "custom_http"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(customHTTPWebhook.Handler),
				),
			),
		),
	)

	r.Handle("/github",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "github"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(github.Handler),
				),
			),
		),
	)

	r.Handle("/gitlab",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "gitlab"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(gitlabWebhook.Handler),
				),
			),
		),
	)

	r.Handle("/metrics", promhttp.Handler())

	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	srv := &http.Server{
		Handler:      loggedRouter,
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting down")
	os.Exit(0)
}
