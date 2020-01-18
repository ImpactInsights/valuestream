package sources

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/cmd/vsperformancereport/metrics"
	"github.com/gocarina/gocsv"
	"github.com/google/go-github/github"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
	"net/http"
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
	return &cli.Command{
		Name:  "github",
		Usage: "generate report on github data",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "event-type",
				Value: "pull-requests",
				Usage: "the event type to generate a report on",
			},
			&cli.StringFlag{
				Name:  "org",
				Value: "",
				Usage: "the organization",
			},
			&cli.StringFlag{
				Name:  "repo",
				Value: "",
				Usage: "an individual repo",
			},
			&cli.IntFlag{
				Name:  "per-page",
				Value: 100,
				Usage: "number of results to pull per page",
			},
			&cli.IntFlag{
				Name:  "max-page",
				Value: 1000000,
				Usage: "max page to pull results from",
			},
			&cli.StringFlag{
				Name:    "access-token",
				Value:   "",
				Usage:   "the event type to generate a report on",
				EnvVars: []string{"VS_PERF_REPORT_GITHUB_ACCESS_TOKEN"},
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			org := c.String("org")
			repo := c.String("repo")
			accessToken := c.String("access-token")
			maxPage := c.Int("max-page")

			var tc *http.Client
			if accessToken != "" {
				ts := oauth2.StaticTokenSource(
					&oauth2.Token{AccessToken: accessToken},
				)
				tc = oauth2.NewClient(ctx, ts)
			}

			client := github.NewClient(tc)

			page := 1

			var metrics []metrics.PullRequestPerformanceMetric
			for {
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

				limiter := time.Tick(500 * time.Millisecond)
				for _, pr := range prs {
					// get the individual PR
					directPR, _, err := client.PullRequests.Get(ctx, org, repo, pr.GetNumber())
					if err != nil {
						return err
					}

					metrics = append(metrics, NewPullRequestPerformanceMetric(directPR))
					<-limiter
				}

				if page == maxPage || page == resp.LastPage {
					break
				}

				page++
			}

			csvString, err := gocsv.MarshalString(metrics)
			if err != nil {
				return err
			}

			fmt.Println(csvString)
			return nil
		},
	}
}
