// +build service

package http

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
var customHTTPPath string

var urlEnvVar string = "TEST_EVENTS_URL"
var customHTTPPathEnvVar string = "TEST_EVENTS_CUSTOM_HTTP_PATH"

func init() {
	ok := true
	baseURL, ok = os.LookupEnv(urlEnvVar)
	if !ok {
		panic(fmt.Sprintf("requires: %q", urlEnvVar))
	}
	customHTTPPath, ok = os.LookupEnv(customHTTPPathEnvVar)
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
		Name:                  "deploy",
		StartEventPath:        "fixtures/events/start.json",
		EndEventPath:          "fixtures/events/end.json",
		ExpectedOperationName: "deploy",
		ExpectedTags: map[string]interface{}{
			"error":   false,
			"service": "customhttp",
			"key1":    "val1",
			"key2":    "val2",
		},
	},
}

func TestServiceEvent_CustomHTTP(t *testing.T) {
	client := &http.Client{}
	u, err := url.Parse(baseURL + customHTTPPath)
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
			assert.Equal(t, tt.ExpectedOperationName, spans[0].Span.OperationName)
			assert.Equal(t, tt.ExpectedTags, spans[0].Tags)
		})
	}
}
