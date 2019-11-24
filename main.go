package main

import (
	"context"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/github"
	"github.com/ImpactInsights/valuestream/eventsources/gitlab"
	customhttp "github.com/ImpactInsights/valuestream/eventsources/http"
	"github.com/ImpactInsights/valuestream/eventsources/jenkins"
	"github.com/ImpactInsights/valuestream/eventsources/jiracloud"
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
			urlPath   string
			name      string
			builderFn func(*cli.Context, opentracing.Tracer) (eventsources.EventSource, error)
		}{
			{
				urlPath:   "/github",
				name:      "github",
				builderFn: github.NewFromCLI,
			},
			{
				urlPath:   "/gitlab",
				name:      "gitlab",
				builderFn: gitlab.NewFromCLI,
			},
			{
				urlPath:   "/customhttp",
				name:      "customhttp",
				builderFn: customhttp.NewFromCLI,
			},
			{
				urlPath:   "/jenkins",
				name:      "jenkins",
				builderFn: jenkins.NewFromCLI,
			},
			{
				urlPath:   "/jira",
				name:      "jira",
				builderFn: jiracloud.NewFromCLI,
			},
		}

		r := mux.NewRouter()

		spans, err := traces.NewBufferedSpanStore(1000)
		if err != nil {
			return err
		}

		for _, s := range sources {
			log.Infof("initializing source: %q", s.name)

			tracer, closer, err := initialzeTracer(ctx, s.name)
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

		r.Handle("/metrics", promhttp.Handler())
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
