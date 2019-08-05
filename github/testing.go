package github

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
)

type StubTracer struct {
	ReturnValue error
	calls       int
}

func (st *StubTracer) handleEvent(ctx context.Context, e interface{}) error {
	st.calls++
	return st.ReturnValue
}

func PostEvent(payload []byte, eventType string, u *url.URL, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest(
		"POST",
		u.String()+"/",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Github-Event", eventType)

	resp, err := client.Do(req)
	return resp, err
}
