package jenkins

import (
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/eventsources/types"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

type BuildState int

type ScmInfo struct {
	URL    string  `json:"url"`
	Commit string  `json:"commit"`
	Branch *string `json:"branch,omitempty"`
}

type BuildEvent struct {
	QueueTime       int               `json:"queueTime"`
	Result          string            `json:"result"`
	CiURL           string            `json:"ciUrl"`
	ContextID       int               `json:"contextId"`
	FullJobName     string            `json:"fullJobName"`
	Parameters      map[string]string `json:"parameters"`
	BuildURL        string            `json:"buildUrl"`
	BuildCause      string            `json:"buildCause"`
	StartTime       int               `json:"startTime"`
	Number          int               `json:"number"`
	StartedUsername string            `json:"startedUsername"`
	JobName         string            `json:"jobName"`
	SlaveInfo       struct {
		SlaveName string `json:"slaveName"`
		Executor  string `json:"executor"`
		Label     string `json:"label"`
	} `json:"slaveInfo"`
	ScmInfo       *ScmInfo `json:"scmInfo,omitempty"`
	StartedUserID string   `json:"startedUserId"`
	Duration      int      `json:"duration"`
	EndTime       int      `json:"endTime"`
}

func (be BuildEvent) Timings() (eventsources.EventTimings, error) {
	return eventsources.EventTimings{}, nil
}

func (be BuildEvent) SpanID() (string, error) {
	id := strings.Join([]string{
		eventsources.TracePrefix,
		sourceName,
		be.OperationName(),
		be.JobName,
		strconv.Itoa(be.Number),
	}, "-")
	log.Debugf("jenkins.BuildEvent.SpanID(): %q", id)

	return id, nil
}

func (be BuildEvent) branchID() *string {
	if be.ScmInfo.Branch == nil {
		return nil
	}
	branch := *be.ScmInfo.Branch
	if strings.HasPrefix(branch, "origin/") {
		branch = strings.TrimPrefix(branch, "origin/")
	}
	return &branch
}

func (be BuildEvent) State(prev *eventsources.EventState) (eventsources.SpanState, error) {
	if be.Result == "INPROGRESS" {
		return eventsources.StartState, nil
	}

	return eventsources.EndState, nil
}

// OperationName determines if the event is a `deploy` or a `build`
// based on the presence of a `type:deploy` tag.
func (be BuildEvent) OperationName() string {
	op := types.BuildEventType

	if strings.HasPrefix(be.JobName, "deploy:") {
		op = types.DeployEventType
	}

	if v, ok := be.Parameters["type"]; ok {
		if v == types.DeployEventType {
			op = types.DeployEventType
		}
	}

	log.WithFields(log.Fields{
		"parameters":     be.Parameters,
		"operation_name": op,
	}).Debug("jenkins.Event.OperationName()")

	return op
}

func (be BuildEvent) IsError() (bool, error) {
	return be.Result != "SUCCESS", nil
}

// ParentSpanID inspects the Jenkins Event payload to
// determine what the parent span is.
// First will check to see if the parent is explicitly specified
// in build params
// Then will check if the build is part of SCM
func (be BuildEvent) ParentSpanID() (*string, error) {
	id, found := be.Parameters["vstrace-trace-id"]
	if found {
		return &id, nil
	}

	return be.branchID(), nil
}

func (be BuildEvent) String() (string, error) {
	b, err := json.Marshal(be)
	return string(b), err
}

func (be BuildEvent) Tags() (map[string]interface{}, error) {
	tags := make(map[string]interface{})
	tags["service"] = "jenkins"
	tags["build.result"] = be.Result
	tags["build.ci.url"] = be.CiURL
	tags["build.context_id"] = be.ContextID
	tags["build.url"] = be.BuildURL
	tags["build.job.name"] = be.JobName
	tags["build.job.full_name"] = be.FullJobName
	tags["build.cause"] = be.BuildCause
	tags["build.number"] = be.Number
	tags["build.started.user.name"] = be.StartedUsername
	tags["build.started.user.id"] = be.StartedUserID

	if be.ScmInfo != nil {
		tags["scm.head.url"] = be.ScmInfo.URL
		tags["scm.head.sha"] = be.ScmInfo.Commit
		tags["scm.branch"] = be.branchID()
	}

	for k, v := range be.Parameters {
		tags[fmt.Sprintf("build.parameter.%s", k)] = v
	}
	return tags, nil
}
