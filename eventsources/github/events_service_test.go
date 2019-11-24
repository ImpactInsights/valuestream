// +build service

package github

import (
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
var githubPath string

var urlEnvVar string = "TEST_EVENTS_URL"
var githubPathEnvVar string = "TEST_EVENTS_GITHUB_PATH"

func init() {
	ok := true
	baseURL, ok = os.LookupEnv(urlEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", urlEnvVar))
	}
	githubPath, ok = os.LookupEnv(githubPathEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", urlEnvVar))
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
		Name:                  "issue",
		StartEventPath:        "fixtures/events/issue/opened.json",
		EndEventPath:          "fixtures/events/issue/closed.json",
		ExpectedOperationName: "issue",
		ExpectedTags: map[string]interface{}{
			"error":                    false,
			"issue.comments_count":     float64(0),
			"issue.number":             float64(36),
			"issue.project.name":       "valuestream",
			"issue.url":                "https://api.github.com/repos/ImpactInsights/valuestream/issues/36",
			"scm.repository.full_name": "ImpactInsights/valuestream",
			"scm.repository.id":        float64(1.97483389e+08),
			"scm.repository.name":      "valuestream",
			"scm.repository.private":   false,
			"scm.repository.url":       "https://api.github.com/repos/ImpactInsights/valuestream",
			"service":                  "github",
			"user.id":                  float64(5.3025024e+07),
			"user.name":                "",
			"user.url":                 "https://api.github.com/users/ImpactInsights",
		},
	},
	{
		Name:                  "pull_request",
		StartEventPath:        "fixtures/events/pull_request/opened.json",
		EndEventPath:          "fixtures/events/pull_request/closed.json",
		ExpectedOperationName: "pull_request",
		ExpectedTags: map[string]interface{}{
			"error":                    false,
			"scm.base.label":           "ImpactInsights:master",
			"scm.base.ref":             "master",
			"scm.base.repo.full_name":  "ImpactInsights/valuestream",
			"scm.base.repo.id":         float64(1.97483389e+08),
			"scm.base.repo.name":       "valuestream",
			"scm.base.repo.private":    false,
			"scm.base.sha":             "3ce9c2fdf2b4deac68762ff58cbdd7f0335b1121",
			"scm.head.label":           "ImpactInsights:feature/github-event-source",
			"scm.head.ref":             "feature/github-event-source",
			"scm.head.sha":             "0b9b63f89b5b5e4fcc9d5d1d9bc873e44add5007",
			"scm.repository.full_name": "ImpactInsights/valuestream",
			"scm.repository.name":      "valuestream",
			"scm.repository.private":   false,
			"scm.repository.url":       "https://api.github.com/repos/ImpactInsights/valuestream",
			"service":                  "github",
			"user.id":                  float64(321963),
			"user.name":                "",
			"user.url":                 "https://api.github.com/users/dm03514",
		},
	},
}

func TestServiceEvent_Github(t *testing.T) {
	client := &http.Client{}
	u, err := url.Parse(baseURL + githubPath)
	assert.NoError(t, err)

	var te *eventsources.TestEvent

	for _, tt := range eventTests {
		t.Run(tt.Name, func(t *testing.T) {
			// reset the tracer
			resp, err := http.Get(baseURL + "/mocktracer/reset")
			assert.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			eventPaths := []string{
				tt.StartEventPath,
				tt.EndEventPath,
			}
			for _, eventPath := range eventPaths {
				te, err = eventsources.NewTestEventFromFixturePath(eventPath)

				rawPayload, err := json.Marshal(te.Payload)
				assert.NoError(t, err)

				eventResp, err := PostEvent(
					rawPayload,
					te.Headers["X-GitHub-Event"],
					u,
					client,
				)

				assert.NoError(t, err)
				eventResp.Body.Close()
				assert.Equal(t, http.StatusOK, eventResp.StatusCode)
			}

			spansResp, err := http.Get(baseURL + "/mocktracer/finished-spans")
			assert.NoError(t, err)

			bs, err := ioutil.ReadAll(spansResp.Body)
			assert.NoError(t, err)
			spansResp.Body.Close()

			assert.Equal(t, http.StatusOK, spansResp.StatusCode)

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
