package gitlab

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	"github.com/ImpactInsights/valuestream/traces"
	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
	"regexp"
	"strconv"
	"strings"
)

const service string = "gitlab"

type IssueEvent struct {
	*gitlab.IssueEvent
}

func (ie IssueEvent) OperationName() string {
	return "issue"
}

func (ie IssueEvent) SpanID() (string, error) {
	if ie.ObjectAttributes.ID == 0 {
		return "", fmt.Errorf("event does not contain Issue.ID")
	}

	return strings.Join([]string{
		types.IssueEventType,
		"gitlab",
		ie.Project.Name,
		strconv.Itoa(ie.ObjectAttributes.ID),
	}, "-"), nil
}

func (ie IssueEvent) State() (eventsources.SpanState, error) {
	if ie.ObjectAttributes.State == "" {
		return eventsources.UnknownState, fmt.Errorf("event does not contain action")
	}

	state := ie.ObjectAttributes.State

	log.Debugf("event state: %q", state)

	if state == "opened" || state == "reopened" {
		return eventsources.StartState, nil
	}

	if state == "closed" {
		return eventsources.EndState, nil
	}

	return eventsources.IntermediaryState, nil
}

func (ie IssueEvent) IsError() (bool, error) {
	return false, nil
}

func (ie IssueEvent) ParentSpanID() (*string, error) {
	return nil, nil
}

func (ie IssueEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["service"] = service

	tags["project.name"] = ie.Project.Name
	tags["project.namespace"] = ie.Project.Namespace
	tags["project.path_with_namespace"] = ie.Project.PathWithNamespace
	tags["project.url"] = ie.Project.URL
	tags["project.visibility"] = ie.Project.Visibility

	if ie.Repository != nil {
		tags["scm.repository.url"] = ie.Repository.URL
		tags["scm.repository.name"] = ie.Repository.Namespace
		tags["scm.repository.full_name"] = ie.Repository.PathWithNamespace
		tags["scm.repository.visibility"] = ie.Repository.Visibility
	}

	tags["issue.id"] = ie.ObjectAttributes.ID
	tags["issue.number"] = ie.ObjectAttributes.IID
	tags["issue.url"] = ie.ObjectAttributes.URL
	if ie.ObjectAttributes.MilestoneID != 0 {
		tags["issue.milestone_id"] = ie.ObjectAttributes.MilestoneID
	}
	tags["issue.branch_name"] = ie.ObjectAttributes.BranchName

	return tags, nil
}

func (ie IssueEvent) TraceID() (*string, error) {
	// vstrace-github-{{repository.name}}-{{issue.number}}
	if ie.Repository == nil || ie.ObjectAttributes.ID == 0 {
		return nil, nil
	}

	traceID := traces.PrefixWith(
		types.IssueEventType,
		fmt.Sprintf("vstrace-gitlab-%s-%d", ie.Repository.Name, ie.ObjectAttributes.IID),
	)

	log.Debugf("gitlab.IssueEvent.TraceID(): trace: %q", traceID)
	return &traceID, nil
}

type MergeEvent struct {
	*gitlab.MergeEvent
}

func (me MergeEvent) OperationName() string {
	return "pull_request"
}

func (me MergeEvent) SpanID() (string, error) {
	return strings.Join([]string{
		types.PullRequestEventType,
		service,
		strconv.Itoa(me.Project.ID),
		strconv.Itoa(me.ObjectAttributes.ID),
	}, "-"), nil
}

func (me MergeEvent) TraceID() (*string, error) {
	return nil, nil
}

func (me MergeEvent) State() (eventsources.SpanState, error) {
	if me.ObjectAttributes.State == "" {
		return eventsources.UnknownState, fmt.Errorf("event does not contain action")
	}

	state := me.ObjectAttributes.State

	log.Debugf("event state: %q", state)

	if state == "opened" || state == "reopened" {
		return eventsources.StartState, nil
	}

	if state == "closed" {
		return eventsources.EndState, nil
	}

	return eventsources.IntermediaryState, nil
}

func (me MergeEvent) IsError() (bool, error) {
	return false, nil
}

func (me MergeEvent) ParentSpanID() (*string, error) {
	r, _ := regexp.Compile("vstrace-[0-9A-Za-z]+-[0-9A-Za-z-]+-[0-9]+")
	matches := r.FindStringSubmatch(me.ObjectAttributes.Description)
	log.Debugf("MergeEvent.ParentSpanID() matches %+v", matches)
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

func (me MergeEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["service"] = service

	tags["project.name"] = me.Project.Name
	tags["project.namespace"] = me.Project.Namespace
	tags["project.path_with_namespace"] = me.Project.PathWithNamespace
	tags["project.url"] = me.Project.WebURL
	tags["project.visibility"] = me.Project.Visibility

	tags["pull_request.id"] = me.ObjectAttributes.IID

	tags["scm.base.label"] = me.ObjectAttributes.SourceBranch
	tags["scm.target.label"] = me.ObjectAttributes.TargetBranch

	return tags, nil
}
