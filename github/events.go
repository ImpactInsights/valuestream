package github

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/google/go-github/github"
	"regexp"
	"strconv"
)

type IssuesEvent struct {
	*github.IssuesEvent
}

// ID identifies the issue event by its github Issue.ID
func (ie IssuesEvent) ID() string {
	return strconv.Itoa(int(*ie.Issue.ID))
}

func (ie IssuesEvent) State() traces.SpanState {
	action := *ie.Action

	if action == "opened" || action == "reopened" {
		return traces.StartState
	}

	if action == "closed" {
		return traces.EndState
	}

	return traces.IntermediaryState
}

func (ie IssuesEvent) Tags() map[string]interface{} {
	tags := make(map[string]interface{})
	if ie.Repo != nil {
		tags["scm.repository.id"] = ie.Repo.GetID()
		tags["scm.repository.url"] = ie.Repo.GetURL()
		tags["scm.repository.name"] = ie.Repo.GetName()
		tags["scm.repository.full_name"] = ie.Repo.GetFullName()
		tags["scm.repository.private"] = ie.Repo.GetPrivate()
	}

	if ie.Issue != nil {
		tags["issue.number"] = ie.Issue.GetNumber()
		tags["issue.url"] = ie.Issue.GetURL()
		tags["issue.comments_count"] = ie.Issue.GetComments()

		if ie.Issue.User != nil {
			tags["user.name"] = ie.Issue.User.GetName()
			tags["user.id"] = ie.Issue.User.GetID()
			tags["user.url"] = ie.Issue.User.GetURL()
		}
	}

	tags["service"] = "github"

	return tags
}

func (ie IssuesEvent) TraceID() (string, bool) {
	// vstrace-github-{{repository.name}}-{{issue.number}}
	if ie.Repo == nil || ie.Repo.Name == nil || ie.Issue == nil || ie.Issue.Number == nil {
		return "", false
	}
	return traces.PrefixISSUE(
		fmt.Sprintf("vstrace-github-%s-%d", ie.Repo.GetName(), ie.Issue.GetNumber()),
	), true
}

type PREvent struct {
	*github.PullRequestEvent
}

func (pr PREvent) BranchRef() *string {
	if pr.GetPullRequest() == nil {
		return nil
	}
	if pr.PullRequest.GetHead() == nil {
		return nil
	}
	res := traces.PrefixSCM(*pr.PullRequest.Head.Ref)
	return &res
}

func (pr PREvent) ID() string {
	return strconv.Itoa(int(*pr.PullRequest.ID))
}

func (pr PREvent) Tags() map[string]interface{} {
	tags := make(map[string]interface{})
	tags["service"] = "github"

	if pr.GetPullRequest() != nil {
		if pr.PullRequest.GetUser() != nil {
			tags["user.name"] = pr.PullRequest.User.GetName()
			tags["user.id"] = pr.PullRequest.User.GetID()
			tags["user.url"] = pr.PullRequest.User.GetURL()
		}

		if pr.PullRequest.GetHead() != nil {
			tags["scm.head.label"] = pr.PullRequest.Head.GetLabel()
			tags["scm.head.ref"] = pr.PullRequest.Head.GetRef()
			tags["scm.head.sha"] = pr.PullRequest.Head.GetSHA()
		}

		if pr.PullRequest.GetBase() != nil {
			tags["scm.base.label"] = pr.PullRequest.Base.GetLabel()
			tags["scm.base.ref"] = pr.PullRequest.Base.GetRef()
			tags["scm.base.sha"] = pr.PullRequest.Base.GetSHA()

			if pr.PullRequest.Base.GetRepo() != nil {
				tags["scm.base.repo.id"] = pr.PullRequest.Base.Repo.GetID()
				tags["scm.base.repo.name"] = pr.PullRequest.Base.Repo.GetName()
				tags["scm.base.repo.full_name"] = pr.PullRequest.Base.Repo.GetFullName()
				tags["scm.base.repo.private"] = pr.PullRequest.Base.Repo.GetPrivate()
			}
		}
	}

	if pr.GetRepo() != nil {
		tags["scm.repository.url"] = pr.Repo.GetURL()
		tags["scm.repository.name"] = pr.Repo.GetName()
		tags["scm.repository.full_name"] = pr.Repo.GetFullName()
		tags["scm.repository.private"] = pr.Repo.GetPrivate()
	}

	return tags
}

// ParentSpanID inspects the PullRequestEvent payload for any references to a parent trace
func (pr PREvent) ParentSpanID() (string, bool) {
	r, _ := regexp.Compile("vstrace-[0-9A-Za-z]+-[0-9A-Za-z]+-[0-9]+")
	matches := r.FindStringSubmatch(*pr.PullRequest.Head.Ref)
	if len(matches) == 0 {
		return "", false
	}
	return traces.PrefixISSUE(matches[0]), true
}

func (pr PREvent) State() traces.SpanState {
	action := *pr.Action

	if action == "opened" || action == "reopened" {
		return traces.StartState
	}

	if action == "closed" {
		return traces.EndState
	}

	return traces.IntermediaryState
}
