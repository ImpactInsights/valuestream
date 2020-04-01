package sources

import (
	"context"
	"fmt"
	"github.com/ImpactInsights/valuestream/cmd/vsperformancereport/metrics"
	vsgh "github.com/ImpactInsights/valuestream/eventsources/github"
	"github.com/gocarina/gocsv"
	"strconv"

	// "github.com/gocarina/gocsv"
	"github.com/shurcooL/githubv4"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io"
	"os"
	"os/signal"
	"time"
)

type Conf struct {
	Client       *githubv4.Client
	Org          string
	Repo         string
	ReposPerPage int
	PrsPerPage   int
	PrState      string
	MaxRepos     int
	MaxPRs       int
	Limiter      *time.Ticker
	Out          io.WriteCloser
	SignalChan   chan os.Signal
}

func (c *Conf) Close() error {
	return c.Out.Close()
}

func NewConf(ctx context.Context, c *cli.Context) (*Conf, error) {
	accessToken := c.String("access-token")
	outFilePath := c.String("out")

	// get the output file
	out := os.Stdout
	if outFilePath != "STDOUT" {
		var err error
		out, err = os.Create(outFilePath)
		if err != nil {
			return nil, err
		}
	}
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	conf := Conf{
		Client:       vsgh.NewClient(ctx, accessToken),
		Out:          out,
		Org:          c.String("org"),
		Repo:         c.String("repo"),
		PrsPerPage:   c.Int("prs-per-page"),
		PrState:      c.String("pr-state"),
		MaxRepos:     c.Int("max-repos"),
		MaxPRs:       c.Int("max-prs"),
		ReposPerPage: c.Int("repos-per-page"),
		Limiter:      time.NewTicker(c.Duration("wait-between-requests")),
		SignalChan:   signalChan,
	}

	return &conf, nil
}

func PullRequests(ctx context.Context, conf *Conf, repos []vsgh.Repository) ([]metrics.PullRequestPerformanceMetric, error) {
	var q vsgh.PullRequestForRepoQueryV4

	var metrics []metrics.PullRequestPerformanceMetric

	for _, repo := range repos {
		variables := map[string]interface{}{
			"owner": githubv4.String(conf.Org),
			"repo":  githubv4.String(repo.Name),
			"state": []githubv4.PullRequestState{
				githubv4.PullRequestState(conf.PrState),
			},
			"prsPerPage": githubv4.Int(conf.PrsPerPage),
			"prsCursor":  (*githubv4.String)(nil),
		}

		page := 1

		for {
			if err := conf.Client.Query(context.Background(), &q, variables); err != nil {
				return metrics, err
			}

			log.WithFields(log.Fields{
				"page":    page,
				"is_last": !q.Repository.PullRequests.PageInfo.HasNextPage,
				"repo":    repo.Name,
				"org":     repo.Login,
			}).Infof("PullRequests.List")

			for _, pr := range q.Repository.PullRequests.Nodes {
				metrics = append(metrics, NewPullRequestPerformanceMetric(repo, pr))
			}

			if !q.HasNextPage() {
				break
			}

			if page*conf.PrsPerPage == conf.MaxPRs {
				break
			}

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("ctx Done()")
			case <-conf.Limiter.C:
			case <-conf.SignalChan:
				return metrics, fmt.Errorf("killed")
			}

			variables["prsCursor"] = q.Repository.PullRequests.PageInfo.EndCursor

			page++
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx Done()")
		case <-conf.Limiter.C:
			continue
		case <-conf.SignalChan:
			return metrics, fmt.Errorf("killed")
		}
	}
	return metrics, nil
}

func Repos(ctx context.Context, conf *Conf) ([]vsgh.Repository, error) {
	// get all repos first
	variables := map[string]interface{}{
		"owner":        githubv4.String(conf.Org),
		"reposPerPage": githubv4.Int(conf.ReposPerPage),
		"reposCursor":  (*githubv4.String)(nil),
	}

	page := 1
	var q vsgh.ReposQueryV4
	var repos []vsgh.Repository

	for {
		if err := conf.Client.Query(context.Background(), &q, variables); err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"page":    page,
			"is_last": !q.HasNextPage(),
		}).Infof("Repositories.List")

		for _, repo := range q.Organization.Repositories.Nodes {
			repos = append(repos, vsgh.Repository{
				Name:  repo.Name,
				Login: repo.Owner.Login,
			})
		}

		if page*conf.ReposPerPage == conf.MaxRepos {
			break
		}

		if !q.HasNextPage() {
			break
		}

		variables["reposCursor"] = q.Organization.Repositories.PageInfo.EndCursor

		page++

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx Done()")
		case <-conf.Limiter.C:
			continue
		case <-conf.SignalChan:
			return nil, fmt.Errorf("killed")
		}
	}

	return repos, nil

}

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
		ID:           strconv.FormatInt(int64(pr.Number), 10),
	}

	// if this was merged use the mergedAt - CreatedAt
	lastAction := time.Now().UTC()

	if pr.Merged {
		lastAction = pr.MergedAt
	} else if pr.Closed {
		lastAction = pr.ClosedAt
	}

	m.DurationSeconds = lastAction.Sub(pr.CreatedAt).Seconds()
	if m.Comments > 0 {
		m.DurationPerComment = m.DurationSeconds / float64(m.Comments)
	}
	m.DurationPerLine = m.DurationSeconds / float64(m.TotalChanges)

	return m
}

func NewGithubCommand() *cli.Command {
	command := &cli.Command{
		Name:  "github",
		Usage: "generate report on github data",
		Subcommands: []*cli.Command{
			{
				Name: "pull-requests",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "repo",
						Value: nil,
						Usage: "an individual repo or * for all repos",
					},
					&cli.DurationFlag{
						Name:  "wait-between-requests",
						Value: 5000 * time.Millisecond,
						Usage: "Duration to wait between requests, defaults to 2.5seconds",
					},
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
						Name:  "prs-per-page",
						Value: 100,
						Usage: "number of pr results to pull per page",
					},
					&cli.IntFlag{
						Name:  "repos-per-page",
						Value: 100,
						Usage: "number of repos results to pull per page",
					},
					&cli.IntFlag{
						Name:  "max-repos",
						Value: 500,
						Usage: "Max number of repo records to pull",
					},
					&cli.IntFlag{
						Name:  "max-prs",
						Value: 100,
						Usage: "Max number of pr records to pull",
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
				Action: func(c *cli.Context) error {
					ctx := context.Background()

					conf, err := NewConf(ctx, c)
					if err != nil {
						return err
					}

					defer conf.Close()

					var repos []vsgh.Repository
					for _, s := range c.StringSlice("repo") {
						repos = append(repos, vsgh.Repository{
							Name:  s,
							Login: conf.Org,
						})
					}

					if len(repos) == 1 && repos[0].Name == "*" {
						repos, err = Repos(ctx, conf)
						if err != nil {
							return err
						}
					}

					metrics, err := PullRequests(ctx, conf, repos)

					if err := gocsv.Marshal(metrics, conf.Out); err != nil {
						return err
					}

					return nil
				},
			},
		},
	}

	return command
}
