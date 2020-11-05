package github

import (
	"context"
	"fmt"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

func NewClient(ctx context.Context, accessToken string, enterpriseDomain string) *githubv4.Client {
	var tc *http.Client
	if accessToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		tc = oauth2.NewClient(ctx, ts)
	}

	if enterpriseDomain == "" {
		return githubv4.NewClient(tc)
	}

	return githubv4.NewEnterpriseClient(
		fmt.Sprintf("https://%s/api/graphql",
			enterpriseDomain,
		),
		tc,
	)

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
	UpdatedAt time.Time
	Merged    bool
	Closed    bool
	Comments  struct {
		TotalCount int
	}
	Additions int
	Deletions int
	Author    struct {
		Login string
	}
	Url            string
	Title          string
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				User struct {
					Login string
				} `graphql:"... on User"`
			}
		}
	} `graphql:"reviewRequests(first: 10)"`
}

func (p PullRequest) Reviewers() string {
	var reviewers []string
	for i := 0; i < len(p.ReviewRequests.Nodes); i++ {
		r := p.ReviewRequests.Nodes[i].RequestedReviewer
		reviewers = append(reviewers, r.User.Login)
	}

	return strings.Join(reviewers, "|")
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
