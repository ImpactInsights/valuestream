package github

import (
	"bytes"
	"net/http"
	"net/url"
)

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
