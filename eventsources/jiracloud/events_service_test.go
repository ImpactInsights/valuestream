// +build service

package jiracloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources"
	"github.com/ImpactInsights/valuestream/tracers"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
)

var baseURL string
var jiraPath string

var urlEnvVar string = "TEST_EVENTS_URL"
var jiraPathEnvVar string = "TEST_EVENTS_JIRA_PATH"

func init() {
	ok := true
	baseURL, ok = os.LookupEnv(urlEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", urlEnvVar))
	}
	jiraPath, ok = os.LookupEnv(jiraPathEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", jiraPathEnvVar))
	}
}

var eventTests = []struct {
	Name                  string
	StartEventPath        string
	EndEventPath          string
	ExpectedOperationName string
	ExpectedTags          map[string]interface{}
}{
	{
		Name:                  "sprint_complete",
		StartEventPath:        "fixtures/events/sprints/started.json",
		EndEventPath:          "fixtures/events/sprints/closed.json",
		ExpectedOperationName: "sprint",
		ExpectedTags: map[string]interface{}{
			"error":                  false,
			"sprint.end_date":        "2019-12-06T17:13:00Z",
			"sprint.id":              float64(1),
			"sprint.name":            "TS Sprint 1",
			"sprint.origin_board_id": float64(2),
			"sprint.start_date":      "2019-11-22T17:13:15.221Z",
			"state":                  "active",
		},
	},
	{
		Name:                  "kanban_issue_in_progress",
		StartEventPath:        "fixtures/events/issues/kanban/selected_for_dev.json",
		EndEventPath:          "fixtures/events/issues/kanban/in_progress.json",
		ExpectedOperationName: "issue",
		ExpectedTags: map[string]interface{}{
			"issue.status.name":   "Selected for Development",
			"issue.type.name":     "Story",
			"project.id":          "10000",
			"user.account_type":   "atlassian",
			"issue.key":           "TP-3",
			"issue.priority.id":   "3",
			"error":               false,
			"issue.priority.name": "Medium",
			"issue.status.id":     "10001",
			"issue.type.id":       "10001",
			"project.key":         "TP",
			"project.name":        "test-project",
			"user.account_id":     "5dd5c77403eda50ef3873efd",
			"user.display_name":   "Daniel Mican",
			"issue.id":            "10002",
		},
	},
	{
		Name:                  "kanban_selected_to_backlog",
		StartEventPath:        "fixtures/events/issues/kanban/selected_for_dev.json",
		EndEventPath:          "fixtures/events/issues/kanban/selected_to_backlog.json",
		ExpectedOperationName: "issue",
		ExpectedTags: map[string]interface{}{
			"issue.priority.id":   "3",
			"project.key":         "TP",
			"user.account_id":     "5dd5c77403eda50ef3873efd",
			"issue.key":           "TP-3",
			"issue.type.name":     "Story",
			"project.name":        "test-project",
			"issue.id":            "10002",
			"project.id":          "10000",
			"issue.status.name":   "Selected for Development",
			"issue.type.id":       "10001",
			"user.account_type":   "atlassian",
			"user.display_name":   "Daniel Mican",
			"error":               false,
			"issue.priority.name": "Medium",
			"issue.status.id":     "10001",
		},
	},
	{
		Name:                  "kanban_in_progress_to_done",
		StartEventPath:        "fixtures/events/issues/kanban/in_progress.json",
		EndEventPath:          "fixtures/events/issues/kanban/done.json",
		ExpectedOperationName: "issue",
		ExpectedTags: map[string]interface{}{
			"user.display_name":   "Daniel Mican",
			"issue.priority.id":   "3",
			"issue.status.name":   "In Progress",
			"issue.type.id":       "10001",
			"user.account_id":     "5dd5c77403eda50ef3873efd",
			"user.account_type":   "atlassian",
			"issue.priority.name": "Medium",
			"issue.id":            "10002",
			"project.key":         "TP",
			"project.name":        "test-project",
			"error":               false,
			"issue.key":           "TP-3",
			"issue.status.id":     "3",
			"issue.type.name":     "Story",
			"project.id":          "10000",
		},
	},
}

func TestServiceEvent_JiraCloud(t *testing.T) {
	client := &http.Client{}
	u, err := url.Parse(baseURL + jiraPath)
	assert.NoError(t, err)

	var te *eventsources.TestEvent

	for _, tt := range eventTests {
		t.Run(tt.Name, func(t *testing.T) {
			// reset the tracer
			resp, err := http.Get(baseURL + "/mocktracer/reset")
			assert.NoError(t, err)
			defer func() {
				err := resp.Body.Close()
				assert.NoError(t, err)
			}()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			eventPaths := []string{
				tt.StartEventPath,
				tt.EndEventPath,
			}
			for _, eventPath := range eventPaths {
				te, err = eventsources.NewTestEventFromFixturePath(eventPath)
				assert.NoError(t, err)

				rawPayload, err := json.Marshal(te.Payload)
				assert.NoError(t, err)

				req, err := http.NewRequest(
					"POST",
					u.String(),
					bytes.NewReader(rawPayload),
				)
				eventResp, err := client.Do(req)

				assert.NoError(t, err)
				eventResp.Body.Close()
				assert.Equal(t, http.StatusOK, eventResp.StatusCode)
			}

			spansResp, err := http.Get(baseURL + "/mocktracer/finished-spans")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, spansResp.StatusCode)

			bs, err := ioutil.ReadAll(spansResp.Body)
			assert.NoError(t, err)
			spansResp.Body.Close()

			var spans []tracers.TestSpan

			err = json.Unmarshal(bs, &spans)
			assert.NoError(t, err)

			assert.Equal(t, 1, len(spans))
			fmt.Printf("%+v\n", spans)

			/*
				for k, v := range spans[0].Tags {
					fmt.Printf("%q: %q,\n", k, v)
				}
			*/

			assert.Equal(t, tt.ExpectedOperationName, spans[0].Span.OperationName)
			assert.Equal(t, tt.ExpectedTags, spans[0].Tags)
		})
	}
}
