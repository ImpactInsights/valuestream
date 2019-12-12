package github

import (
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPullRequestEvent_ParentSpanID(t *testing.T) {
	branchName := "feature/vstrace-github-pull_request-valuestream-1/hi"

	traceIDTests := []struct {
		name     string
		pr       PREvent
		expected string
	}{
		{
			name: "branch_name_contains",
			pr: PREvent{
				&github.PullRequestEvent{
					PullRequest: &github.PullRequest{
						Head: &github.PullRequestBranch{
							Ref: &branchName,
						},
					},
				},
			},
			expected: "vstrace-github-pull_request-valuestream-1",
		},
	}
	for _, tt := range traceIDTests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := tt.pr.ParentSpanID()
			assert.NoError(t, err)
			assert.NotNil(t, match)
			assert.Equal(t, tt.expected, *match)
		})
	}
}

func TestIssuesEvent_Duration(t *testing.T) {
	created := time.Date(
		2000, 12, 1, 0, 0, 0, 0, time.UTC,
	)
	end := time.Date(
		2000, 12, 1, 1, 0, 0, 0, time.UTC,
	)
	closed := "closed"
	ie := IssuesEvent{
		&github.IssuesEvent{
			Action: &closed,
			Issue: &github.Issue{
				CreatedAt: &created,
				ClosedAt:  &end,
			},
		},
	}
	timings, err := ie.Timings()
	assert.NoError(t, err)
	assert.Equal(t, float64(1), (*timings.Duration).Hours())
}

func TestPREvent_Duration(t *testing.T) {
	created := time.Date(
		2000, 12, 1, 0, 0, 0, 0, time.UTC,
	)
	end := time.Date(
		2000, 12, 1, 1, 0, 0, 0, time.UTC,
	)
	closed := "closed"
	pr := PREvent{
		&github.PullRequestEvent{
			Action: &closed,
			PullRequest: &github.PullRequest{
				CreatedAt: &created,
				ClosedAt:  &end,
			},
		},
	}
	timings, err := pr.Timings()
	assert.NoError(t, err)
	assert.Equal(t, float64(1), (*timings.Duration).Hours())
}
