package github

import (
	"bytes"
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/google/go-github/github"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEventTracer_WebhookHandler_IssueOpen(t *testing.T) {
	event := []byte(
		`{
	"action": "opened",
	"issue": {
		"number": 1
	},
	"repository": {
		"name": "valuestream"
	}
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")

	tracer := mocktracer.New()
	github := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	webhook := NewWebhook(github, nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 1, github.traces.Count())
}

func TestEventTracer_WebhookHandler_IssueClose(t *testing.T) {
	event := []byte(
		`{
		"action": "closed",
		"issue": {
            "number": 1
		},
		"repository": {
			"name": "valuestream"
        }
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "issues")

	tracer := mocktracer.New()
	github := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	github.traces.Set(
		traces.PrefixISSUE("vstrace-github-valuestream-1"),
		tracer.StartSpan("issue"),
	)

	rr := httptest.NewRecorder()
	webhook := NewWebhook(github, nil)
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 0, github.traces.Count())
}

func TestEventTracer_handleIssue_EndStateNoStartFound(t *testing.T) {
	tracer := mocktracer.New()
	gh := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	closed := "closed"
	name := "name"
	number := 1
	err := gh.handleIssue(&github.IssuesEvent{
		Action: &closed,
		Repo: &github.Repository{
			Name: &name,
		},
		Issue: &github.Issue{
			Number: &number,
		},
	})
	assert.IsType(t, traces.SpanMissingError{}, err)
}

func TestEventTracer_WebhookHandler_PullRequestOpen(t *testing.T) {
	event := []byte(
		`{
		"action": "opened",
		"pull_request": {
			"id": 1,
			"head": {
				"ref": ""
 			}
		}
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Github-Event", "pull_request")

	tracer := mocktracer.New()
	github := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	webhook := NewWebhook(github, nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 1, github.spans.Count())
}

func TestEventTracer_WebhookHandler_PullRequestClose(t *testing.T) {
	event := []byte(
		`{
	"action": "closed",
	"pull_request": {
		"id": 1,
		"head": {
			"ref": ""
		}
	}
}`)
	req, err := http.NewRequest("GET", "/github", bytes.NewReader(event))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Github-Event", "pull_request")

	tracer := mocktracer.New()
	github := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	github.spans.Set("1", tracer.StartSpan("pull_request"))
	webhook := NewWebhook(github, nil)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webhook.Handler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Equal(t, 0, github.spans.Count())
}

func TestNewEventTracer_handlePullRequest_TracePresent(t *testing.T) {
	tracer := mocktracer.New()
	gh := NewEventTracer(
		tracer,
		traces.NewMemoryUnboundedSpanStore(),
		traces.NewMemoryUnboundedSpanStore(),
	)
	rootSpan := tracer.StartSpan("issue")
	gh.traces.Set(
		traces.PrefixISSUE("vstrace-github-valuestream-1"),
		rootSpan,
	)
	branchName := "feature/vstrace-github-valuestream-1/hi"
	openedAction := "opened"
	id := int64(1)

	err := gh.handlePullRequest(&github.PullRequestEvent{
		Action: &openedAction,
		PullRequest: &github.PullRequest{
			ID: &id,
			Head: &github.PullRequestBranch{
				Ref: &branchName,
			},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, gh.spans.Count())
	span, ok := gh.spans.Get("1")
	assert.True(t, ok)
	s := span.(*mocktracer.MockSpan)
	assert.Equal(t, "pull_request", s.OperationName)

	// check that this ParentID is the rootSpan TraceID
	rs := rootSpan.(*mocktracer.MockSpan)
	assert.Equal(t, rs.SpanContext.SpanID, s.ParentID)
}
