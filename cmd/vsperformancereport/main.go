package main

import (
	"github.com/ImpactInsights/valuestream/cmd/vsperformancereport/metrics"
	"github.com/ImpactInsights/valuestream/cmd/vsperformancereport/sources"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Usage = "generates a performance report from a given event source"
	app.Commands = []*cli.Command{
		sources.NewGithubCommand(),
		metrics.NewPullRequestAggregation(),
	}
	app.Action = func(c *cli.Context) error {
		log.Debug("here")
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
