package github

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/google/go-github/github"
	"regexp"
	"strconv"
)

type IssuesEvent struct {
	*github.IssuesEvent
}

func (ie IssuesEvent) OperationName() string {
	return "issue"
}

// ID identifies the issue event by its github Issue.ID
func (ie IssuesEvent) SpanID() (string, error) {
	if ie.Issue == nil || ie.Issue.ID == nil {
		return "", fmt.Errorf("event does not contain Issue.ID")
	}

	return traces.PrefixWith(
		types.IssueEventType,
		strconv.Itoa(int(*ie.Issue.ID)),
	), nil
}

func (ie IssuesEvent) State() (webhooks.SpanState, error) {
	if ie.Action == nil {
		return webhooks.UnknownState, fmt.Errorf("event does not contain action")
	}

	action := *ie.Action

	if action == "opened" || action == "reopened" {
		return webhooks.StartState, nil
	}

	if action == "closed" {
		return webhooks.EndState, nil
	}

	return webhooks.IntermediaryState, nil
}

func (ie IssuesEvent) IsError() (bool, error) {
	return false, nil
}

// TODO - Issues can reference other issues inside their body
// to model 'epics' or issues of issues
func (ie IssuesEvent) ParentSpanID() (*string, error) {
	return nil, nil
}

func (ie IssuesEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	if ie.Repo != nil {
		tags["scm.repository.id"] = ie.Repo.GetID()
		tags["scm.repository.url"] = ie.Repo.GetURL()
		tags["scm.repository.name"] = ie.Repo.GetName()
		tags["issue.project.name"] = ie.Repo.GetName()
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

	return tags, nil
}

func (ie IssuesEvent) TraceID() (*string, error) {
	// vstrace-github-{{repository.name}}-{{issue.number}}
	if ie.Repo == nil || ie.Repo.Name == nil || ie.Issue == nil || ie.Issue.Number == nil {
		return nil, nil
	}
	traceID := traces.PrefixWith(
		types.IssueEventType,
		fmt.Sprintf("vstrace-github-%s-%d", ie.Repo.GetName(), ie.Issue.GetNumber()),
	)
	return &traceID, nil
}

type PREvent struct {
	*github.PullRequestEvent
}

func (pr PREvent) OperationName() string {
	return "pull_request"
}

func (pr PREvent) TraceID() (*string, error) {
	return pr.BranchRef(), nil
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

func (pr PREvent) SpanID() (string, error) {
	if pr.PullRequest == nil || pr.PullRequest.ID == nil {
		return "", fmt.Errorf("event must contain pull request id")
	}
	prID := *pr.PullRequest.ID
	return strconv.Itoa(int(prID)), nil
}

func (pr PREvent) Tags() (map[string]interface{}, error) {
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

	return tags, nil
}

// ParentSpanID inspects the PullRequestEvent payload for any references to a parent trace
func (pr PREvent) ParentSpanID() (*string, error) {
	r, _ := regexp.Compile("vstrace-[0-9A-Za-z]+-[0-9A-Za-z]+-[0-9]+")
	matches := r.FindStringSubmatch(*pr.PullRequest.Head.Ref)
	if len(matches) == 0 {
		return nil, nil
	}
	// TODO the type needs to be included in the trace in order
	// to support referencing multiple different types....

	id := traces.PrefixWith(
		types.IssueEventType,
		matches[0],
	)
	return &id, nil
}

func (pr PREvent) State() (webhooks.SpanState, error) {
	if pr.Action == nil {
		return webhooks.UnknownState, fmt.Errorf("event does not contain action")
	}
	action := *pr.Action

	if action == "opened" || action == "reopened" {
		return webhooks.StartState, nil
	}

	if action == "closed" {
		return webhooks.EndState, nil
	}

	return webhooks.IntermediaryState, nil
}

func (pr PREvent) IsError() (bool, error) {
	return false, nil
}
