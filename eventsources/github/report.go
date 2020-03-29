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
		} `graphql:"pullRequests(states: $state, first: $perPage, orderBy: {field: UPDATED_AT, direction: DESC}, after: $commentsCursor)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type PullRequestQueryV4 struct {
	Organization struct {
		Repositories struct {
			TotalCount int
			PageInfo   struct {
				EndCursor   string
				HasNextPage bool
			}
			Edges []struct {
				Cursor string
				Node   struct {
					Name string
				}
			}
		} `graphql:"repositories(first: 100)"`
	} `graphql:"organization(login: $login)"`
}
