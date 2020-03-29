package github

import (
	"context"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"net/http"
	"time"
)

func NewClient(ctx context.Context, accessToken string) *githubv4.Client {
	var tc *http.Client
	if accessToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		tc = oauth2.NewClient(ctx, ts)
	}

	return githubv4.NewClient(tc)
}

/*
{
  organization(login: $orgName) {
    repositories(first: 10) {
      totalCount
      edges {
    		cursor
        node {
          name
          pullRequests(states: CLOSED, last: 100) {
            nodes {
              number
              mergedAt
              comments {
                totalCount
              }
              additions
              deletions
            }
          }
        }
      }
      pageInfo {
        endCursor
        hasNextPage
      }
    }
  }
}
*/

type PullRequest struct {
	Number    int
	CreatedAt time.Time
	MergedAt  time.Time
	ClosedAt  time.Time
	Merged    bool
	Closed    bool
	Comments  struct {
		TotalCount int
	}
	Additions int
	Deletions int
}

type Repository struct {
	Name  string
	Login string
}

type PullRequestForRepoQueryV4 struct {
	Repository struct {
		Name  string
		Owner struct {
			Login string
		}
		PullRequests struct {
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
			Nodes []PullRequest
		} `graphql:"pullRequests(states: $state, first: $prsPerPage, orderBy: {field: UPDATED_AT, direction: DESC}, after: $prsCursor)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func (p PullRequestForRepoQueryV4) HasNextPage() bool {
	return p.Repository.PullRequests.PageInfo.HasNextPage
}

type ReposQueryV4 struct {
	Organization struct {
		Repositories struct {
			TotalCount int
			Nodes      []struct {
				Name  string
				Owner struct {
					Login string
				}
			}
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"repositories(first: $reposPerPage, after: $reposCursor)"`
	} `graphql:"organization(login: $owner)"`
}

func (p ReposQueryV4) HasNextPage() bool {
	return p.Organization.Repositories.PageInfo.HasNextPage
}

type PullRequestQueryV4 struct {
	Organization struct {
		Repositories struct {
			TotalCount int
			PageInfo   struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
			Edges []struct {
				Cursor string
				Node   struct {
					Name         string
					PullRequests struct {
						Nodes []PullRequest
					} `graphql:"pullRequests(states: $state, first: $prsPerPage, orderBy: {field: UPDATED_AT, direction: DESC})"`
				}
			}
		} `graphql:"repositories(first: $reposPerPage, after: $reposCursor)"`
	} `graphql:"organization(login: $owner)"`
}

func (p PullRequestQueryV4) HasNextPage() bool {
	return p.Organization.Repositories.PageInfo.HasNextPage
}
