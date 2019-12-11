package main

import (
	"context"
	"contrib.go.opencensus.io/exporter/prometheus"
	"fmt"
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
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/plugin/runmetrics"
	"go.opencensus.io/stats/view"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const envLogLevel string = "VS_LOG_LEVEL"

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

		spans, err := traces.NewBufferedSpanStore(1000)
		if err != nil {
			return err
		}

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
				ochttp.WithRouteTag(
					http.HandlerFunc(webhook.Handler),
					s.urlPath,
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

		exporter, err := prometheus.NewExporter(prometheus.Options{
			Namespace: "vs",
		})

		if err != nil {
			return fmt.Errorf("failed to create the Prometheus exporter: %v", err)
		}

		r.Handle("/metrics", exporter)

		if err := view.Register(
			ochttp.ServerRequestCountView,
			ochttp.ServerRequestBytesView,
			ochttp.ServerResponseBytesView,
			ochttp.ServerLatencyView,
			ochttp.ServerRequestCountByMethod,
			ochttp.ServerResponseCountByStatusCode,
			webhooks.EventStartCountView,
			webhooks.EventEndCountView,
		); err != nil {
			return fmt.Errorf("failed to register ochttp Server views: %v", err)
		}

		view.SetReportingPeriod(500 * time.Millisecond)

		if err := runmetrics.Enable(runmetrics.RunMetricOptions{
			EnableCPU:    true,
			EnableMemory: true,
		}); err != nil {
			return err
		}

		loggedRouter := handlers.LoggingHandler(os.Stdout, r)

		srv := &http.Server{
			Handler: &ochttp.Handler{
				Handler:     loggedRouter,
				Propagation: &b3.HTTPFormat{},
			},
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
