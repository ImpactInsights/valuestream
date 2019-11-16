package github

import (
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	"testing"
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
