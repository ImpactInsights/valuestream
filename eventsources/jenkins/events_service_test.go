// +build service

package jenkins

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
var jenkinsPath string

var urlEnvVar = "TEST_EVENTS_URL"
var jenkinsPathEnvVar = "TEST_EVENTS_JENKINS_PATH"

func init() {
	ok := true
	baseURL, ok = os.LookupEnv(urlEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", urlEnvVar))
	}
	jenkinsPath, ok = os.LookupEnv(jenkinsPathEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", jenkinsPathEnvVar))
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
		Name:                  "build_aborted",
		StartEventPath:        "fixtures/events/build/inprogress.json",
		EndEventPath:          "fixtures/events/build/aborted.json",
		ExpectedOperationName: "build",
		ExpectedTags: map[string]interface{}{
			"build.cause":                "Started by anonymous",
			"build.ci.url":               "http://localhost:8080/jenkins/",
			"build.context_id":           float64(1.02490963e+08),
			"build.job.full_name":        "jenkins_test",
			"build.job.name":             "jenkins_test",
			"build.number":               float64(252),
			"build.parameter.someParams": "someValue",
			"build.result":               "INPROGRESS",
			"build.started.user.id":      "anonymous",
			"build.started.user.name":    "anonymous",
			"build.url":                  "aUrl",
			"error":                      true,
			"scm.branch":                 "master",
			"scm.head.sha":               "aCommitHash",
			"scm.head.url":               "aGithubUrl",
			"service":                    "jenkins",
		},
	},
	{
		Name:                  "build_success",
		StartEventPath:        "fixtures/events/build/inprogress.json",
		EndEventPath:          "fixtures/events/build/success.json",
		ExpectedOperationName: "build",
		ExpectedTags: map[string]interface{}{
			"build.cause":                "Started by anonymous",
			"build.ci.url":               "http://localhost:8080/jenkins/",
			"build.context_id":           float64(1.02490963e+08),
			"build.job.full_name":        "jenkins_test",
			"build.job.name":             "jenkins_test",
			"build.number":               float64(252),
			"build.parameter.someParams": "someValue",
			"build.result":               "INPROGRESS",
			"build.started.user.id":      "anonymous",
			"build.started.user.name":    "anonymous",
			"build.url":                  "aUrl",
			"error":                      false,
			"scm.branch":                 "master",
			"scm.head.sha":               "aCommitHash",
			"scm.head.url":               "aGithubUrl",
			"service":                    "jenkins",
		},
	},
	{
		Name:                  "deploy_success",
		StartEventPath:        "fixtures/events/deploy/inprogress.json",
		EndEventPath:          "fixtures/events/deploy/success.json",
		ExpectedOperationName: "deploy",
		ExpectedTags: map[string]interface{}{
			"build.cause":                "Started by anonymous",
			"build.ci.url":               "http://localhost:8080/jenkins/",
			"build.context_id":           float64(1.02490963e+08),
			"build.job.full_name":        "jenkins_test",
			"build.job.name":             "deploy:jenkins_test",
			"build.number":               float64(252),
			"build.parameter.someParams": "someValue",
			"build.parameter.type":       "deploy",
			"build.result":               "INPROGRESS",
			"build.started.user.id":      "anonymous",
			"build.started.user.name":    "anonymous",
			"build.url":                  "aUrl",
			"error":                      false,
			"scm.branch":                 "master",
			"scm.head.sha":               "aCommitHash",
			"scm.head.url":               "aGithubUrl",
			"service":                    "jenkins",
		},
	},
}

func TestServiceEvent_Jenkins(t *testing.T) {
	client := &http.Client{}
	u, err := url.Parse(baseURL + jenkinsPath)
	assert.NoError(t, err)

	var te *eventsources.TestEvent

	for _, tt := range eventTests {
		t.Run(tt.Name, func(t *testing.T) {
			// reset the tracer
			resp1, err := http.Get(baseURL + "/mocktracer/reset")
			assert.NoError(t, err)
			defer resp1.Body.Close()
			assert.Equal(t, http.StatusOK, resp1.StatusCode)

			eventPaths := []string{
				tt.StartEventPath,
				tt.EndEventPath,
			}
			for _, eventPath := range eventPaths {
				te, err = eventsources.NewTestEventFromFixturePath(eventPath)
				assert.NoError(t, err)

				fmt.Printf("Path: %q, payload: %q, err: %q\n", eventPath, te, err)

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

			if len(spans) == 1 {
				assert.Equal(t, tt.ExpectedOperationName, spans[0].Span.OperationName)
				assert.Equal(t, tt.ExpectedTags, spans[0].Tags)
			}
		})
	}
}
