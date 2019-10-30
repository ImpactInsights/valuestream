package main

import (
	"context"
	"flag"
	customhttp "github.com/ImpactInsights/valuestream/eventsources/http"
	"github.com/ImpactInsights/valuestream/eventsources/jenkins"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/ImpactInsights/valuestream/github"
	// "github.com/ImpactInsights/valuestream/jenkins"
	"github.com/ImpactInsights/valuestream/tracer"
	"github.com/ImpactInsights/valuestream/traces"
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

	switch *tracerImplName {
	case "jaeger":
		var githubTracerCloser io.Closer
		var jenkinsTracerCloser io.Closer
		var customHTTPTracerCloser io.Closer

		log.Infof("initializing tracer: jaeger")
		githubTracer, githubTracerCloser = tracer.InitJaeger("github")
		defer githubTracerCloser.Close()

		jenkinsTracer, jenkinsTracerCloser = tracer.InitJaeger("jenkins")
		defer jenkinsTracerCloser.Close()

		customHTTPTracer, customHTTPTracerCloser = tracer.InitJaeger("custom_http")
		defer customHTTPTracerCloser.Close()

	case "lightstep":

		log.Infof("initializing tracer: lightstep")

		accessToken := os.Getenv("VS_LIGHTSTEP_ACCESS_TOKEN")
		githubTracer = tracer.InitLightstep("github", accessToken)
		jenkinsTracer = tracer.InitLightstep("jenkins", accessToken)
		customHTTPTracer = tracer.InitLightstep("custom_http", accessToken)

		defer lightstep.Close(ctx, githubTracer)
		defer lightstep.Close(ctx, jenkinsTracer)
		defer lightstep.Close(ctx, customHTTPTracer)

	default:
		log.Infof("initializing tracer: logging")
		githubTracer = tracer.LoggingTracer{}
		jenkinsTracer = tracer.LoggingTracer{}
		customHTTPTracer = tracer.LoggingTracer{}
	}

	ts, err := traces.NewBufferedSpanStore(1000)
	if err != nil {
		panic(err)
	}
	go ts.Monitor(ctx, time.Second*60, "traces")

	jenkinsSpans, err := traces.NewBufferedSpanStore(500)
	if err != nil {
		panic(err)
	}
	go jenkinsSpans.Monitor(ctx, time.Second*60, "jenkins")

	jenkinsSource, err := jenkins.NewSource(jenkinsTracer)
	if err != nil {
		panic(err)
	}

	jenkins, err := webhooks.New(
		jenkinsSource,
		nil,
		ts,
		jenkinsSpans,
	)
	if err != nil {
		panic(err)
	}

	githubSpans, err := traces.NewBufferedSpanStore(500)
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

	customHTTP, err := customhttp.NewSource(customHTTPTracer)
	if err != nil {
		panic(err)
	}

	customHTTPSpans, err := traces.NewBufferedSpanStore(500)
	if err != nil {
		panic(err)
	}
	go customHTTPSpans.Monitor(ctx, time.Second*20, customHTTP.Name())

	customHTTPWebhook, err := webhooks.New(
		customHTTP,
		nil,
		ts,
		customHTTPSpans,
	)

	r := mux.NewRouter()

	r.Handle("/jenkins/",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "jenkins_build"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(jenkins.Handler),
				),
			),
		),
	)

	r.Handle("/customhttp/",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "custom_http"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(customHTTPWebhook.Handler),
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
