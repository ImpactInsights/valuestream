package main

import (
	"context"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/github"
	"github.com/ImpactInsights/valuestream/eventsources/gitlab"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/ImpactInsights/valuestream/tracers"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
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
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "addr",
			Value:  "127.0.0.1:5000",
			Usage:  "address to start valuestream server",
			EnvVar: "VS_ADDR",
		},
		cli.StringFlag{
			Name:   "tracer, t",
			Value:  "logging",
			Usage:  "tracer implementation to use: 'logger|jaeger|lightstep'",
			EnvVar: "VS_TRACER_BACKEND",
		},
		cli.StringFlag{
			Name:   "tracer-access-token",
			Value:  "",
			Usage:  "Tracer access token",
			EnvVar: "VS_TRACER_ACCESS_TOKEN",
		},
	}
	app.Action = func(c *cli.Context) error {
		ctx := context.Background()
		// get the tracer
		initialzeTracer := tracers.InitializerFromCLI(c, c.String("tracer"))

		sources := []struct {
			urlPath    string
			tracerName string
			builderFn  func(*cli.Context, opentracing.Tracer) (eventsources.EventSource, error)
		}{
			{
				urlPath:    "/github",
				tracerName: "github",
				builderFn:  github.NewFromCLI,
			},
			{
				urlPath:    "/gitlab",
				tracerName: "gitlab",
				builderFn:  gitlab.NewFromCLI,
			},
		}

		r := mux.NewRouter()

		spans, err := traces.NewBufferedSpanStore(1000)
		if err != nil {
			return err
		}

		for _, s := range sources {
			log.Infof("initializing source: %q", s.tracerName)

			tracer, closer, err := initialzeTracer(ctx, s.tracerName)
			if err != nil {
				return err
			}

			defer func() {
				if err := closer.Close(); err != nil {
					panic(err)
				}
			}()

			source, err := s.builderFn(c, tracer)
			if err != nil {
				return err
			}
			webhook, err := webhooks.New(
				source,
				tracers.NewRequestScopedUsingSources(),
				spans,
			)

			r.Handle(s.urlPath,
				promhttp.InstrumentHandlerDuration(
					duration.MustCurryWith(prometheus.Labels{"handler": source.Name()}),
					promhttp.InstrumentHandlerCounter(counter,
						promhttp.InstrumentHandlerResponseSize(responseSize,
							http.HandlerFunc(webhook.Handler),
						),
					),
				),
			)
		}

		if c.String("tracer") == "mock" {
			tracer, _, err := initialzeTracer(ctx, "global")
			if err != nil {
				return err
			}

			if err := tracers.Register(tracer.(*mocktracer.MockTracer), spans, r); err != nil {
				return err
			}
		}

		loggedRouter := handlers.LoggingHandler(os.Stdout, r)

		srv := &http.Server{
			Handler:      loggedRouter,
			Addr:         c.String("addr"),
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		go func() {
			log.Infof("Starting Server: %q", c.String("addr"))
			if err := srv.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()

		waitForShutdown(srv)

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	/*

	var addr = flag.String("addr", "127.0.0.1:5000", "addr/port for the tes")
	var tracerImplName = flag.String("tracer", "logging", "tracer implementation to use: 'logger|jaeger|lightstep'")
	flag.Parse()

	ctx := context.Background()
	var githubTracer opentracing.Tracer
	var jenkinsTracer opentracing.Tracer
	var customHTTPTracer opentracing.Tracer
	var gitlabTracer opentracing.Tracer
	var jiraTracer opentracing.Tracer

	globalTracer := mocktracer.New()


	switch *tracerImplName {
	case "jaeger":
		var githubTracerCloser io.Closer
		var jenkinsTracerCloser io.Closer
		var customHTTPTracerCloser io.Closer
		var gitlabTracerCloser io.Closer
		var jiraTracerCloser io.Closer

		log.Infof("initializing tracer: jaeger")
		githubTracer, githubTracerCloser = tracers.InitJaeger("github")
		defer githubTracerCloser.Close()

		jenkinsTracer, jenkinsTracerCloser = tracers.InitJaeger("jenkins")
		defer jenkinsTracerCloser.Close()

		customHTTPTracer, customHTTPTracerCloser = tracers.InitJaeger("custom_http")
		defer customHTTPTracerCloser.Close()

		gitlabTracer, gitlabTracerCloser = tracers.InitJaeger("gitlab")
		defer gitlabTracerCloser.Close()

		jiraTracer, jiraTracerCloser = tracers.InitJaeger("jira")
		defer jiraTracerCloser.Close()

	case "mock":
		githubTracer = globalTracer
		jenkinsTracer = globalTracer
		customHTTPTracer = globalTracer
		gitlabTracer = globalTracer
		jiraTracer = globalTracer

	case "lightstep":
		log.Infof("initializing tracer: lightstep")

		accessToken := os.Getenv("VS_LIGHTSTEP_ACCESS_TOKEN")
		githubTracer = tracers.InitLightstep("github", accessToken)
		jenkinsTracer = tracers.InitLightstep("jenkins", accessToken)
		customHTTPTracer = tracers.InitLightstep("custom_http", accessToken)
		gitlabTracer = tracers.InitLightstep("gitlab", accessToken)
		jiraTracer = tracers.InitLightstep("jira", accessToken)

		defer lightstep.Close(ctx, githubTracer)
		defer lightstep.Close(ctx, jenkinsTracer)
		defer lightstep.Close(ctx, customHTTPTracer)
		defer lightstep.Close(ctx, gitlabTracer)
		defer lightstep.Close(ctx, jiraTracer)

	default:
		log.Infof("initializing tracer: logging")
		githubTracer = tracers.LoggingTracer{}
		jenkinsTracer = tracers.LoggingTracer{}
		customHTTPTracer = tracers.LoggingTracer{}
		gitlabTracer = tracers.LoggingTracer{}
		jiraTracer = tracers.LoggingTracer{}
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
	if err != nil {
		panic(err)
	}

	jira, err := jiracloud.NewSource(jiraTracer)
	if err != nil {
		panic(err)
	}

	jiraWebhook, err := webhooks.New(
		jira,
		tracers.NewRequestScopedUsingSources(),
		nil,
		spans,
	)
	if err != nil {
		panic(err)
	}

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

	r.Handle("/jira",
		promhttp.InstrumentHandlerDuration(
			duration.MustCurryWith(prometheus.Labels{"handler": "jira"}),
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize,
					http.HandlerFunc(jiraWebhook.Handler),
				),
			),
		),
	)

	r.Handle("/metrics", promhttp.Handler())

	if *tracerImplName == "mock" {
		if err := tracers.Register(globalTracer, spans, r); err != nil {
			panic(err)
		}
	}

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
	*/
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
