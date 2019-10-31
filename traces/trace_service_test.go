// +build !unit

package traces

import (
	"bytes"
	"encoding/json"
	"fmt"
	gh "github.com/ImpactInsights/valuestream/github"
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"os"
	"path"
	"testing"
)

var githubPath = "github"
var jenkinsPath = "jenkins"

func init() {
	if value, ok := os.LookupEnv("VS_TEST_GITHUB_PATH"); ok {
		githubPath = value
	}

	if value, ok := os.LookupEnv("VS_TEST_JENKINS_PATH"); ok {
		jenkinsPath = value
	}
}

func closePullRequest(t *testing.T, githubURL *url.URL, client *http.Client) {
	branchName := "vstrace-github-valuestream-1"
	action := "closed"
	id := int64(1)
	pr := &github.PullRequestEvent{
		Action: &action,
		PullRequest: &github.PullRequest{
			ID: &id,
			Head: &github.PullRequestBranch{
				Ref: &branchName,
			},
		},
	}
	payload, err := json.Marshal(pr)
	assert.NoError(t, err)
	resp, err := gh.PostEvent(payload, "pull_request", githubURL, client)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func createPullRequest(t *testing.T, githubURL *url.URL, client *http.Client) {
	branchName := "vstrace-github-valuestream-1"
	openedAction := "opened"
	id := int64(1)
	pr := &github.PullRequestEvent{
		Action: &openedAction,
		PullRequest: &github.PullRequest{
			ID: &id,
			Head: &github.PullRequestBranch{
				Ref: &branchName,
			},
		},
	}
	payload, err := json.Marshal(pr)
	assert.NoError(t, err)
	resp, err := gh.PostEvent(payload, "pull_request", githubURL, client)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func openIssue(t *testing.T, githubURL *url.URL, client *http.Client) {
	repoName := "valuestream"
	action := "opened"
	id := int64(1)
	number := 1
	i := &github.IssuesEvent{
		Action: &action,
		Repo: &github.Repository{
			Name: &repoName,
		},
		Issue: &github.Issue{
			ID:     &id,
			Number: &number,
		},
	}

	payload, err := json.Marshal(i)
	assert.NoError(t, err)
	resp, err := gh.PostEvent(payload, "issues", githubURL, client)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func closeIssue(t *testing.T, githubURL *url.URL, client *http.Client) {
	repoName := "valuestream"
	action := "closed"
	id := int64(1)
	number := 1
	i := &github.IssuesEvent{
		Action: &action,
		Repo: &github.Repository{
			Name: &repoName,
		},
		Issue: &github.Issue{
			ID:     &id,
			Number: &number,
		},
	}

	payload, err := json.Marshal(i)
	assert.NoError(t, err)
	resp, err := gh.PostEvent(payload, "issues", githubURL, client)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func createJenkinsBuild(t *testing.T, jenkinsURL *url.URL, client *http.Client) {
	event := []byte(`
{
	"result": "INPROGRESS",
	"buildUrl": "aUrl",
	"jobName": "createJenkinsBuild",
	"parameters": {
		"vstrace-trace-id": "vstrace-github-valuestream-1"
    }
}`)

	req, err := http.NewRequest(
		"POST",
		jenkinsURL.String()+"/",
		bytes.NewReader(event),
	)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func createJenkinsDeploy(t *testing.T, jenkinsURL *url.URL, client *http.Client) {
	event := []byte(`
{
	"result": "INPROGRESS",
	"buildUrl": "aUrl",
	"jobName": "createJenkinsBuild",
	"parameters": {
		"vstrace-trace-id": "vstrace-github-valuestream-1",
		"type": "deploy"
    }
}`)

	req, err := http.NewRequest(
		"POST",
		jenkinsURL.String()+"/",
		bytes.NewReader(event),
	)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func jenkinsCIBuild(t *testing.T, jenkinsURL *url.URL, client *http.Client, result string) {
	event := []byte(fmt.Sprintf(`{
	"result": "%s",
	"buildUrl": "aUrl",
	"jobName": "jenkinsCIBuild",
	"scmInfo": {
		"branch": "origin/vstrace-github-valuestream-1"
    }
}`, result))

	req, err := http.NewRequest(
		"POST",
		jenkinsURL.String()+"/",
		bytes.NewReader(event),
	)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func finishJenkinsBuild(t *testing.T, jenkinsURL *url.URL, client *http.Client) {
	event := []byte(`
{
	"result": "SUCCESS",
	"buildUrl": "aUrl"
}`)

	req, err := http.NewRequest(
		"POST",
		jenkinsURL.String()+"/",
		bytes.NewReader(event),
	)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGithubJenkinsTrace(t *testing.T) {
	URL, err := url.Parse(os.Getenv("VALUESTREAM_URL"))
	assert.NoError(t, err)

	client := &http.Client{}

	githubURL := &url.URL{}
	*githubURL = *URL
	githubURL.Path = path.Join(githubURL.Path, githubPath, "/")

	jenkinsURL := &url.URL{}
	*jenkinsURL = *URL
	jenkinsURL.Path = path.Join(jenkinsURL.Path, jenkinsPath, "/")

	openIssue(t, githubURL, client)
	createPullRequest(t, githubURL, client)
	closePullRequest(t, githubURL, client)
	createJenkinsBuild(t, jenkinsURL, client)
	finishJenkinsBuild(t, jenkinsURL, client)
	closeIssue(t, githubURL, client)
}

func TestGithubJenkinsPRBuildJenkinsDeployTrace(t *testing.T) {
	URL, err := url.Parse(os.Getenv("VALUESTREAM_URL"))
	assert.NoError(t, err)

	client := &http.Client{}

	githubURL := &url.URL{}
	*githubURL = *URL
	githubURL.Path = path.Join(githubURL.Path, githubPath, "/")

	jenkinsURL := &url.URL{}
	*jenkinsURL = *URL
	jenkinsURL.Path = path.Join(jenkinsURL.Path, jenkinsPath, "/")

	openIssue(t, githubURL, client)
	createPullRequest(t, githubURL, client)
	jenkinsCIBuild(t, jenkinsURL, client, "INPROGRESS")
	jenkinsCIBuild(t, jenkinsURL, client, "SUCCESS")
	closePullRequest(t, githubURL, client)
	createJenkinsBuild(t, jenkinsURL, client)
	finishJenkinsBuild(t, jenkinsURL, client)
	createJenkinsDeploy(t, jenkinsURL, client)
	finishJenkinsBuild(t, jenkinsURL, client)
	closeIssue(t, githubURL, client)
}
