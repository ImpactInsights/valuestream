package github

import (
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPullRequestEvent_ParentSpanID(t *testing.T) {
	branchName := "feature/vstrace-github-valuestream-1/hi"

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
			expected: "ISSUE-vstrace-github-valuestream-1",
		},
	}
	for _, tt := range traceIDTests {
		t.Run(tt.name, func(t *testing.T) {
			match, _ := tt.pr.ParentSpanID()
			assert.Equal(t, tt.expected, match)
		})
	}
}
