package github

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/google/go-github/github"
	"strconv"
	"strings"
)

type IssuesEvent struct {
	*github.IssuesEvent
}

func (ie IssuesEvent) OperationName() string {
	return "issue"
}

// ID identifies the issue event by its github Issue.ID
func (ie IssuesEvent) SpanID() (string, error) {
	if ie.Issue == nil || ie.Issue.Number == nil {
		return "", fmt.Errorf("event does not contain Issue.ID")
	}

	if ie.Repo == nil || ie.Repo.Name == nil {
		return "", fmt.Errorf("event does not contain Repo.Name")
	}

	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.IssueEventType,
		ie.Repo.GetName(),
		strconv.Itoa(ie.Issue.GetNumber()),
	}, "-"), nil
}

func (ie IssuesEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	if ie.Action == nil {
		return eventsources.UnknownState, fmt.Errorf("event does not contain action")
	}

	action := *ie.Action

	if action == "opened" || action == "reopened" {
		return eventsources.StartState, nil
	}

	if action == "closed" {
		return eventsources.EndState, nil
	}

	return eventsources.IntermediaryState, nil
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

	tags["service"] = sourceName

	return tags, nil
}

type PREvent struct {
	*github.PullRequestEvent
}

func (pr PREvent) OperationName() string {
	return "pull_request"
}

func (pr PREvent) BranchRef() *string {
	if pr.GetPullRequest() == nil {
		return nil
	}
	if pr.PullRequest.GetHead() == nil {
		return nil
	}

	return pr.PullRequest.Head.Ref
}

func (pr PREvent) SpanID() (string, error) {
	if pr.PullRequest == nil || pr.PullRequest.ID == nil {
		return "", fmt.Errorf("event must contain pull request id")
	}
	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.PullRequestEventType,
		pr.Repo.GetName(),
		strconv.Itoa(int(pr.PullRequest.GetID())),
	}, "-"), nil
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
	matches, err := traces.Matches(pr.PullRequest.Head.GetRef())
	if err != nil {
		return nil, err
	}

	return &matches[0], nil
}

func (pr PREvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	if pr.Action == nil {
		return eventsources.UnknownState, fmt.Errorf("event does not contain action")
	}
	action := *pr.Action

	if action == "opened" || action == "reopened" {
		return eventsources.StartState, nil
	}

	if action == "closed" {
		return eventsources.EndState, nil
	}

	return eventsources.IntermediaryState, nil
}

func (pr PREvent) IsError() (bool, error) {
	return false, nil
}
