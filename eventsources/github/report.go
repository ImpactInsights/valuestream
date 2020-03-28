package github

import (
	"context"
	"encoding/json"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io"
	"net/http"
)

// ability to pull all Pull Requests from a repo

// Need the ability to pull ALL repos or a list of repos

// Rate limit of 5000 requests / hour, so need a state file or something that can be resumed

// Build a list of all Repos and Pull Requests

type PRQueryPlan struct {
	Repos map[string]PullRequestPlan
}


func (p *PRQueryPlan) AddRepo(name string) {
	p.Repos[name] = struct{}{}
}

func NewPRQueryPlan() *PRQueryPlan {
	return &PRQueryPlan{
		Repos: make(map[string]PullRequestPlan),
	}
}

type PullRequestPlan struct {
	Repo string


}

func (p *PRQueryPlan) Write(w io.Writer) error {
	bs, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(bs); err != nil {
		return err
	}

	return nil
}

func NewClient(ctx context.Context, accessToken string) *github.Client {
	var tc *http.Client
	if accessToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		tc = oauth2.NewClient(ctx, ts)
	}

	return github.NewClient(tc)
}
