package jiracloud

import (
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	"github.com/andygrunwald/go-jira"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

type eventType int

const (
	unknownEvent eventType = iota
	sprintEvent
	issueEvent
)

const (
	kanbanBacklog              string = "Backlog"
	kanbanSelectForDevelopment string = "Selected for Development"
	kanbanInProgress           string = "In Progress"
	kanbanDone                 string = "Done"
)

type Event struct {
	Timestamp    int
	WebhookEvent string `json:"webhookEvent"`
}

func (e Event) Type() eventType {
	if strings.HasPrefix(e.WebhookEvent, "sprint_") {
		return sprintEvent
	}

	if strings.HasPrefix(e.WebhookEvent, "jira:issue_") {
		return issueEvent
	}

	return unknownEvent
}

type SprintEvent struct {
	Sprint jira.Sprint
}

func (se SprintEvent) Timings() (eventsources.EventTimings, error) {
	return eventsources.EventTimings{}, nil
}

func (se SprintEvent) SpanID() (string, error) {
	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.SprintEventType,
		strconv.Itoa(se.Sprint.ID),
	}, "-"), nil
}

func (se SprintEvent) OperationName() string {
	return types.SprintEventType
}

func (se SprintEvent) ParentSpanID() (*string, error) {
	return nil, nil
}

// IsError returns true if the sprint ended before its `endDate`
// TODO
func (se SprintEvent) IsError() (bool, error) {
	return false, nil
}

func (se SprintEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	switch se.Sprint.State {
	case "active":
		return eventsources.StartState, nil
	case "closed":
		return eventsources.EndState, nil
	}

	return eventsources.UnknownState, nil
}

func (se SprintEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["state"] = se.Sprint.State
	tags["sprint.id"] = se.Sprint.ID
	tags["sprint.name"] = se.Sprint.Name
	tags["sprint.start_date"] = se.Sprint.StartDate
	tags["sprint.end_date"] = se.Sprint.EndDate
	tags["sprint.origin_board_id"] = se.Sprint.OriginBoardID
	return tags, nil
}

type IssueEvent struct {
	User      jira.User
	Issue     jira.Issue
	Changelog jira.Changelog
}

func (ie IssueEvent) Timings() (eventsources.EventTimings, error) {
	return eventsources.EventTimings{}, nil
}

func (ie IssueEvent) SpanID() (string, error) {
	return strings.Join([]string{
		"vstrace",
		sourceName,
		types.IssueEventType,
		ie.Issue.Key,
	}, "-"), nil
}
func (ie IssueEvent) OperationName() string {
	return types.IssueEventType
}
func (ie IssueEvent) ParentSpanID() (*string, error) {
	return nil, nil
}
func (ie IssueEvent) IsError() (bool, error) {
	return false, nil
}
func (ie IssueEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	if ie.Issue.Fields.Status == nil {
		return eventsources.UnknownState, fmt.Errorf("issue missing status")
	}
	log.WithFields(log.Fields{
		"prev_state":  prev,
		"status.name": ie.Issue.Fields.Status.Name,
		"status.id":   ie.Issue.Fields.Status.ID,
	}).Debugf("jira.issueEvent.State()")

	switch ie.Issue.Fields.Status.Name {
	case kanbanSelectForDevelopment, kanbanInProgress:
		return eventsources.TransitionState, nil

	case kanbanDone, kanbanBacklog: // and from status is missing...
		// need to differentiate between created vs
		// removed, right now we don't track the backlog state
		return eventsources.EndState, nil
	}

	// if the state is unknown could be custom swimlanes assume intermediate
	return eventsources.IntermediaryState, nil
}
func (ie IssueEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})

	tags["user.account_id"] = ie.User.AccountID
	tags["user.account_type"] = ie.User.AccountType
	tags["user.display_name"] = ie.User.DisplayName

	tags["issue.id"] = ie.Issue.ID
	tags["issue.key"] = ie.Issue.Key

	tags["issue.type.id"] = ie.Issue.Fields.Type.ID
	tags["issue.type.name"] = ie.Issue.Fields.Type.Name

	tags["project.id"] = ie.Issue.Fields.Project.ID
	tags["project.key"] = ie.Issue.Fields.Project.Key
	tags["project.name"] = ie.Issue.Fields.Project.Name

	tags["issue.priority.name"] = ie.Issue.Fields.Priority.Name
	tags["issue.priority.id"] = ie.Issue.Fields.Priority.ID

	tags["issue.status.id"] = ie.Issue.Fields.Status.ID
	tags["issue.status.name"] = ie.Issue.Fields.Status.Name

	return tags, nil
}
