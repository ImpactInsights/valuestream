package sources

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/cmd/vsperformancereport/metrics"
	vsgh "github.com/ImpactInsights/valuestream/eventsources/github"
	"github.com/gocarina/gocsv"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func NewPullRequestPerformanceMetric(pr *github.PullRequest) metrics.PullRequestPerformanceMetric {
	// TODO nil checks
	base := pr.GetBase()
	repo := base.GetRepo()
	merged := pr.GetMerged()
	m := metrics.PullRequestPerformanceMetric{
		Owner:        repo.GetOwner().GetLogin(),
		Repo:         repo.GetName(),
		CreatedAt:    pr.GetCreatedAt(),
		Merged:       merged,
		Comments:     pr.GetComments(),
		Additions:    pr.GetAdditions(),
		Deletions:    pr.GetDeletions(),
		TotalChanges: pr.GetAdditions() + pr.GetDeletions(),
	}

	var d time.Duration

	// if this was merged use the mergedAt - CreatedAt
	if merged {
		d = pr.GetMergedAt().Sub(pr.GetCreatedAt())
	} else {
		d = pr.GetClosedAt().Sub(pr.GetCreatedAt())
	}

	m.DurationSeconds = d.Seconds()

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
				Value: "closed",
				Usage: "PRs to query",
			},
		},
		Subcommands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "visibility",
						Value: "private",
					},
					&cli.IntFlag{
						Name:  "max-page",
						Value: 1,
					},
					&cli.StringFlag{
						Name:  "out",
						Value: "STDOUT",
						Usage: "File to write output to",
					},
				},
				Action: func(c *cli.Context) error {
					ctx := context.Background()

					signalChan := make(chan os.Signal, 1)
					signal.Notify(signalChan, os.Interrupt)

					org := c.String("org")
					accessToken := c.String("access-token")
					maxPage := c.Int("max-page")
					outFilePath := c.String("out")

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
					page := 1
					last := "unknown"

					limiter := time.NewTicker(500 * time.Millisecond)
					plan := vsgh.NewPRQueryPlan()
					for {
						log.WithFields(log.Fields{
							"page": page,
							"last": last,
						}).Infof("Repos.List")

						repos, resp, err := client.Repositories.List(
							ctx,
							org,
							&github.RepositoryListOptions{
								Visibility: c.String("visibility"),
								ListOptions: github.ListOptions{
									PerPage: c.Int("per-page"),
									Page:    page,
								},
							},
						)

						last = strconv.Itoa(resp.LastPage)

						if err != nil {
							return err
						}

						fmt.Println(resp.Rate)

						for _, repo := range repos {
							plan.AddRepo(repo.GetName())
						}

						if page == maxPage || page == resp.LastPage {
							log.Infof("max page: %d reached", maxPage)
							break
						}

						page++

						select {
						case <-limiter.C:
							continue
						case <-signalChan:
							return fmt.Errorf("killed")
						}
					}

					plan.Write(out)
					return nil
				},
			},
			{
				Name: "execute",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "event-type",
						Value: "pull-requests",
						Usage: "the event type to generate a report on",
					},
					&cli.StringFlag{
						Name:  "repo",
						Value: "",
						Usage: "an individual repo",
					},
					&cli.IntFlag{
						Name:  "max-page-per-repo",
						Value: 1000000,
						Usage: "max page to pull results from",
					},
				},
				Action: func(c *cli.Context) error {
					ctx := context.Background()

					signalChan := make(chan os.Signal, 1)
					signal.Notify(signalChan, os.Interrupt)

					org := c.String("org")
					repo := c.String("repo")
					accessToken := c.String("access-token")
					maxPage := c.Int("max-page-per-repo")
					outFilePath := c.String("out")

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

					page := 1
					last := "unknown"

					var metrics []metrics.PullRequestPerformanceMetric
					for {
						log.WithFields(log.Fields{
							"page": page,
							"last": last,
						}).Infof("PullRequests.List")

						prs, resp, err := client.PullRequests.List(
							ctx,
							org,
							repo,
							&github.PullRequestListOptions{
								State: "closed",
								ListOptions: github.ListOptions{
									PerPage: c.Int("per-page"),
									Page:    page,
								},
							},
						)
						if err != nil {
							return err
						}
						last = strconv.Itoa(resp.LastPage)

						limiter := time.NewTicker(500 * time.Millisecond)
						for i, pr := range prs {
							log.WithFields(log.Fields{
								"curr": i,
								"last": len(prs),
							}).Infof("PullRequests.Get")

							// get the individual PR
							directPR, _, err := client.PullRequests.Get(ctx, org, repo, pr.GetNumber())
							if err != nil {
								return err
							}

							metrics = append(metrics, NewPullRequestPerformanceMetric(directPR))

							select {
							case <-limiter.C:
								continue
							case <-signalChan:
								return fmt.Errorf("killed")
							}
						}

						if page == maxPage || page == resp.LastPage {
							log.Infof("max page: %d reached", maxPage)
							break
						}

						page++
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
