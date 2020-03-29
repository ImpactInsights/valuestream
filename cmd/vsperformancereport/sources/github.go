package sources

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/cmd/vsperformancereport/metrics"
	vsgh "github.com/ImpactInsights/valuestream/eventsources/github"
	"github.com/gocarina/gocsv"
	"github.com/shurcooL/githubv4"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"time"
)

func NewPullRequestPerformanceMetric(repo vsgh.Repository, pr vsgh.PullRequest) metrics.PullRequestPerformanceMetric {
	// TODO nil checks
	m := metrics.PullRequestPerformanceMetric{
		Owner:        repo.Login,
		Repo:         repo.Name,
		CreatedAt:    pr.CreatedAt,
		Merged:       pr.Merged,
		Comments:     pr.Comments.TotalCount,
		Additions:    pr.Additions,
		Deletions:    pr.Deletions,
		TotalChanges: pr.Additions + pr.Deletions,
	}

	// if this was merged use the mergedAt - CreatedAt
	lastAction := time.Now().UTC()

	if pr.Merged {
		lastAction = pr.MergedAt
	} else if pr.Closed {
		lastAction = pr.ClosedAt
	}

	m.DurationSeconds = lastAction.Sub(pr.CreatedAt).Seconds()
	m.DurationPerComment = float64(m.Comments) / m.DurationSeconds
	m.DurationPerLine = float64(m.TotalChanges) / m.DurationSeconds

	return m
}

func NewGithubCommand() *cli.Command {
	command := &cli.Command{
		Name:  "github",
		Usage: "generate report on github data",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "org",
				Value: "",
				Usage: "the organization",
			},
			&cli.StringFlag{
				Name:    "access-token",
				Value:   "",
				Usage:   "the event type to generate a report on",
				EnvVars: []string{"VS_PERF_REPORT_GITHUB_ACCESS_TOKEN"},
			},
			&cli.IntFlag{
				Name:  "per-page",
				Value: 100,
				Usage: "number of results to pull per page",
			},
			&cli.StringFlag{
				Name:  "pr-state",
				Value: "MERGED",
				Usage: "PRs to query: \"CLOSED|MERGED|OPEN\"",
			},
			&cli.StringFlag{
				Name:  "out",
				Value: "STDOUT",
				Usage: "File to write output to",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name: "pull-requests",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "repo",
						Value: "",
						Usage: "an individual repo",
					},
				},
				Action: func(c *cli.Context) error {
					ctx := context.Background()

					signalChan := make(chan os.Signal, 1)
					signal.Notify(signalChan, os.Interrupt)
					accessToken := c.String("access-token")
					org := c.String("org")
					repo := c.String("repo")
					perPage := c.Int("per-page")
					outFilePath := c.String("out")
					prState := c.String("pr-state")

					// get the output file
					out := os.Stdout
					if outFilePath != "STDOUT" {
						var err error
						out, err = os.Create(outFilePath)
						if err != nil {
							return err
						}
						defer out.Close()
					}

					client := vsgh.NewClient(ctx, accessToken)

					var metrics []metrics.PullRequestPerformanceMetric
					var q vsgh.PullRequestForRepoQueryV4
					variables := map[string]interface{}{
						"owner": githubv4.String(org),
						"repo":  githubv4.String(repo),
						"state": []githubv4.PullRequestState{
							githubv4.PullRequestState(prState),
						},
						"perPage":        githubv4.Int(perPage),
						"commentsCursor": (*githubv4.String)(nil),
					}

					limiter := time.NewTicker(500 * time.Millisecond)
					page := 1
					for {
						if err := client.Query(context.Background(), &q, variables); err != nil {
							return err
						}

						log.WithFields(log.Fields{
							"page":    page,
							"is_last": !q.Repository.PullRequests.PageInfo.HasNextPage,
						}).Infof("PullRequests.List")

						for _, pr := range q.Repository.PullRequests.Nodes {
							metrics = append(metrics, NewPullRequestPerformanceMetric(
								vsgh.Repository{
									Name:  q.Repository.Name,
									Login: q.Repository.Owner.Login,
								},
								pr,
							))
						}

						if !q.Repository.PullRequests.PageInfo.HasNextPage {
							break
						}

						variables["commentsCursor"] = q.Repository.PullRequests.PageInfo.EndCursor
						page++

						select {
						case <-limiter.C:
							continue
						case <-signalChan:
							return fmt.Errorf("killed")
						}
					}

					if err := gocsv.Marshal(metrics, out); err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	return command
}
