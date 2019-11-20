package gitlab

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	"github.com/ImpactInsights/valuestream/traces"
	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
	"strconv"
	"strings"
)

type IssueEvent struct {
	*gitlab.IssueEvent
}

func (ie IssueEvent) OperationName() string {
	return types.IssueEventType
}

func (ie IssueEvent) SpanID() (string, error) {
	if ie.ObjectAttributes.IID == 0 {
		return "", fmt.Errorf("event does not contain Issue.ID")
	}

	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.IssueEventType,
		ie.Project.Name,
		strconv.Itoa(ie.ObjectAttributes.IID),
	}, "-"), nil
}

func (ie IssueEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
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
	tags["service"] = sourceName

	tags["project.name"] = ie.Project.Name
	tags["project.namespace"] = ie.Project.Namespace
	tags["project.path_with_namespace"] = ie.Project.PathWithNamespace
	tags["project.url"] = ie.Project.URL
	tags["project.visibility"] = ie.Project.Visibility
	tags["event.state"] = ie.ObjectAttributes.State
	tags["event.action"] = ie.ObjectAttributes.Action

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

type MergeEvent struct {
	*gitlab.MergeEvent
}

func (me MergeEvent) OperationName() string {
	return types.PullRequestEventType
}

func (me MergeEvent) SpanID() (string, error) {
	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.PullRequestEventType,
		me.Project.Name,
		strconv.Itoa(me.ObjectAttributes.IID),
	}, "-"), nil
}

func (me MergeEvent) TraceID() (*string, error) {
	return nil, nil
}

func (me MergeEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
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
	matches, err := traces.Matches(me.ObjectAttributes.Description)
	if err != nil {
		return nil, err
	}
	log.Debugf("MergeEvent.ParentSpanID() matches %+v", matches)

	if len(matches) == 0 {
		return nil, nil
	}

	return &matches[0], nil
}

func (me MergeEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["service"] = sourceName

	tags["event.state"] = me.ObjectAttributes.State
	tags["event.action"] = me.ObjectAttributes.Action

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

type PipelineEvent struct {
	*gitlab.PipelineEvent
}

func (pe PipelineEvent) OperationName() string {
	return "pipeline"
}

func (pe PipelineEvent) SpanID() (string, error) {
	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.BuildEventType,
		pe.Project.Name,
		strconv.Itoa(pe.ObjectAttributes.ID),
	}, "-"), nil
}

func (pe PipelineEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	state := pe.ObjectAttributes.Status

	if state == "" {
		return eventsources.UnknownState, fmt.Errorf("event does not contain action")
	}

	log.Debugf("event state: %q", state)

	if state == "pending" {
		return eventsources.StartState, nil
	}

	if state == "running" {
		return eventsources.TransitionState, nil
	}

	if state == "canceled" || state == "success" {
		return eventsources.EndState, nil
	}

	return eventsources.IntermediaryState, nil
}

func (pe PipelineEvent) IsError() (bool, error) {
	isErr := false

	status := pe.ObjectAttributes.Status

	if status != "success" && status != "running" {
		isErr = true
	}

	return isErr, nil
}

// ParentSpanID inspects the pipeline payload for the causing event:
// - Pull Request
// - Issue
func (pe PipelineEvent) ParentSpanID() (*string, error) {
	return nil, nil
}

func (pe PipelineEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["service"] = sourceName
	tags["event.type"] = "pipeline"
	tags["event.state"] = pe.ObjectAttributes.Status

	tags["project.id"] = pe.Project.ID
	tags["project.name"] = pe.Project.Name
	tags["project.namespace"] = pe.Project.Namespace
	tags["project.path_with_namespace"] = pe.Project.PathWithNamespace
	tags["project.url"] = pe.Project.WebURL
	tags["project.visibility"] = pe.Project.Visibility

	tags["user.name"] = pe.User.Name
	tags["user.username"] = pe.User.Username

	tags["build.id"] = pe.ObjectAttributes.ID
	tags["build.ref"] = pe.ObjectAttributes.Ref
	tags["build.tag"] = pe.ObjectAttributes.Tag
	tags["build.sha"] = pe.ObjectAttributes.SHA
	tags["build.before_sha"] = pe.ObjectAttributes.BeforeSHA

	sID, _ := pe.SpanID()
	tags["vstrace.span.id"] = sID

	state, _ := pe.State(nil)
	tags["vstrace.state"] = state

	return tags, nil
}

type JobEvent struct {
	*gitlab.JobEvent
}

func (je JobEvent) OperationName() string {
	return types.BuildEventType

}

func (je JobEvent) SpanID() (string, error) {
	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.BuildEventType,
		je.ProjectName,
		strconv.Itoa(je.BuildID),
	}, "-"), nil
}

// State identifies the current state of the build, valid states are:
// - pending
// - created
// - running
// - canceled
// - success
func (je JobEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	if je.BuildStatus == "" {
		return eventsources.UnknownState, fmt.Errorf("event does not contain action")
	}

	state := je.BuildStatus

	log.Debugf("event state: %q", state)

	if state == "pending" || state == "created" {
		return eventsources.StartState, nil
	}

	if state == "running" {
		return eventsources.TransitionState, nil
	}

	if state == "canceled" || state == "success" {
		return eventsources.EndState, nil
	}

	return eventsources.IntermediaryState, nil
}

func (je JobEvent) IsError() (bool, error) {
	isErr := false

	status := je.BuildStatus

	if status != "success" && status != "running" {
		isErr = true
	}

	return isErr, nil
}

func (je JobEvent) ParentSpanID() (*string, error) {
	// TODO
	id := strings.Join([]string{
		"vstrace",
		sourceName,
		types.BuildEventType,
		je.Repository.Name,
		strconv.Itoa(je.Commit.ID),
	}, "-")

	return &id, nil
}

func (je JobEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["service"] = sourceName
	tags["event.state"] = je.BuildStatus

	tags["build.kind"] = je.ObjectKind
	tags["build.ref"] = je.Ref
	tags["build.tag"] = je.Tag
	tags["build.je.ore_sha"] = je.BeforeSHA
	tags["build.sha"] = je.SHA
	tags["build.id"] = je.BuildID
	tags["build.name"] = je.BuildName
	tags["build.stage"] = je.BuildStage
	tags["build.status"] = je.BuildStatus
	tags["build.allow_failure"] = je.BuildAllowFailure
	tags["build.project.id"] = je.ProjectID
	tags["build.project.name"] = je.ProjectName

	tags["user.id"] = je.User.ID
	tags["user.name"] = je.User.Name

	tags["scm.commit.id"] = je.Commit.ID
	tags["scm.commit.sha"] = je.Commit.SHA
	tags["scm.commit.author.name"] = je.Commit.AuthorName
	tags["scm.commit.status"] = je.Commit.Status

	sID, _ := je.SpanID()
	tags["vstrace.span.id"] = sID

	parentID, _ := je.ParentSpanID()
	tags["vstrace.parent.id"] = *parentID

	state, _ := je.State(nil)
	tags["vstrace.state"] = state

	return tags, nil
}
