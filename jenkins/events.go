package jenkins

import (
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/traces"
	"strings"
)

type BuildState int

const (
	startState BuildState = iota
	endState
)

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

func (be BuildEvent) ID() string {
	return be.BuildURL
}

func (be BuildEvent) BranchID() string {
	branch := *be.ScmInfo.Branch
	if strings.HasPrefix(branch, "origin/") {
		return strings.TrimPrefix(branch, "origin/")
	}
	return branch
}

func (be BuildEvent) State() BuildState {
	if be.Result == "INPROGRESS" {
		return startState
	}

	return endState
}

// OperationName determines if the event is a `deploy` or a `build`
// based on the presence of a `type:deploy` tag.
func (be BuildEvent) OperationName() string {
	if v, ok := be.Parameters["type"]; ok {
		if v == "deploy" {
			return "deploy"
		}
	}

	return "build"
}

// ParentSpanID inspects the Jenkins Event payload to
// determine what the parent span is.
// First will check to see if the parent is explicitly specified
// in build params
// Then will check if the build is part of SCM
func (be BuildEvent) ParentSpanID() (string, bool) {
	id, found := be.Parameters["vstrace-trace-id"]
	if found {
		return traces.PrefixISSUE(id), found
	}
	// TODO this needs more intelligent SCM specific parsing
	if be.ScmInfo != nil && be.ScmInfo.Branch != nil {
		return traces.PrefixSCM(be.BranchID()), true
	}

	return "", false
}

func (be BuildEvent) String() (string, error) {
	b, err := json.Marshal(be)
	return string(b), err
}

func (be BuildEvent) Tags() map[string]interface{} {
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
		tags["scm.branch"] = be.BranchID()
	}

	for k, v := range be.Parameters {
		tags[fmt.Sprintf("build.parameter.%s", k)] = v
	}
	return tags
}
