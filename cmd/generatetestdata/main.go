package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources/jenkins"
	gh "github.com/ImpactInsights/valuestream/github"
	"github.com/google/go-github/github"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

const (
	open        string = "opened"
	closed      string = "closed"
	timeoutUnit        = time.Second
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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func buildUsers() []github.User {
	name1 := "user_1"
	id1 := int64(11111111)
	url1 := "https://api.github.com/users/user_1"

	name2 := "user_2"
	id2 := int64(222222)
	url2 := "https://api.github.com/users/user_2"

	return []github.User{
		{
			Name: &name1,
			ID:   &id1,
			URL:  &url1,
		},
		{
			Name: &name2,
			ID:   &id2,
			URL:  &url2,
		},
	}
}

func buildRepos() []github.Repository {
	id1 := int64(1123123)
	url1 := "http://github.com/dm03514/test_1"
	name1 := "test_1"
	fullName1 := "dm03514/test_1"

	id2 := int64(2123123)
	url2 := "http://github.com/dm03514/test_2"
	name2 := "test_2"
	fullName2 := "dm03514/test_2"

	return []github.Repository{
		{
			ID:       &id1,
			URL:      &url1,
			Name:     &name1,
			FullName: &fullName1,
		},
		{
			ID:       &id2,
			URL:      &url2,
			Name:     &name2,
			FullName: &fullName2,
		},
	}
}

func buildJenkinsEvent() jenkins.BuildEvent {
	number := rand.Int()
	return jenkins.BuildEvent{
		Result:      "INPROGRESS",
		CiURL:       "http://jenkins-ci.com/",
		FullJobName: "service-deploy",
		JobName:     "service-deploy",
		Number:      number,
		BuildURL:    fmt.Sprintf("job/service-deploy/%d", number),
		Parameters: map[string]string{
			"type": "deploy",
		},
	}
}

func buildIssuesEvent(repos []github.Repository, users []github.User) *github.IssuesEvent {
	action := open
	id := rand.Intn(100000000000000)
	repoIndex := rand.Intn(2)
	repo := repos[repoIndex]
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d",
		*repo.FullName, id)

	user := rand.Intn(1)
	comments := rand.Intn(5)
	return &github.IssuesEvent{
		Action: &action,
		Repo:   &repo,
		Issue: &github.Issue{
			URL:      &url,
			Comments: &comments,
			Number:   &id,
			User:     &users[user],
		},
	}
}

func buildPullRequestEvent(repos []github.Repository, users []github.User) *github.PullRequestEvent {
	action := open

	id := int64(rand.Intn(100000000000000))
	repoIndex := rand.Intn(2)
	repo := repos[repoIndex]
	user := rand.Intn(2)
	baseCommit := RandStringRunes(32)
	headCommit := RandStringRunes(32)
	baseLabel := "dm03514:master"
	baseRef := "master"

	headLabel := fmt.Sprintf("dm03514:feature/branch-%d", id)
	headRef := fmt.Sprintf("feature/branch-%d", id)

	return &github.PullRequestEvent{
		Action: &action,
		Repo:   &repo,
		PullRequest: &github.PullRequest{
			User: &users[user],
			ID:   &id,
			Base: &github.PullRequestBranch{
				Repo:  &repo,
				SHA:   &baseCommit,
				Label: &baseLabel,
				Ref:   &baseRef,
			},
			Head: &github.PullRequestBranch{
				Ref:   &headRef,
				Label: &headLabel,
				SHA:   &headCommit,
			},
		},
	}
}

func main() {
	var addr = flag.String("addr", "http://localhost:5000", "valuestream target")
	flag.Parse()

	rand.Seed(42)

	URL, err := url.Parse(*addr)
	if err != nil {
		panic(err)
	}

	repos := buildRepos()
	users := buildUsers()

	githubURL := &url.URL{}
	*githubURL = *URL
	githubURL.Path = path.Join(githubURL.Path, githubPath, "/")

	jenkinsURL := &url.URL{}
	*jenkinsURL = *URL
	jenkinsURL.Path = path.Join(jenkinsURL.Path, jenkinsPath, "/")

	client := &http.Client{}

	done := make(chan int)

	// generate data for pr
	go func() {
		// every n minutes generate some duration that lasts m minutes
		ticker := time.NewTicker(1 * timeoutUnit)
		for {
			select {
			case <-ticker.C:
				fmt.Println("pull_request: creating")
				pre := buildPullRequestEvent(repos, users)
				payload, err := json.Marshal(pre)
				if err != nil {
					fmt.Printf("pull_request: error json marshal: %s", err)
					panic(err)
				}

				resp, err := gh.PostEvent(payload, "pull_request", githubURL, client)
				if err != nil {
					panic(err)
				}
				resp.Body.Close()

				num := rand.Intn(10)
				fmt.Printf("pull_request: sleeping: %d %s\n", num, timeoutUnit.String())
				time.Sleep(time.Duration(num) * timeoutUnit)

				action := closed
				pre.Action = &action

				payload, err = json.Marshal(pre)
				if err != nil {
					fmt.Printf("pull_request: error json marshal: %s", err)
					panic(err)
				}

				fmt.Println("pull_request: closing")
				resp, err = gh.PostEvent(payload, "pull_request", githubURL, client)
				if err != nil {
					panic(err)
				}
				resp.Body.Close()
			}
		}
	}()

	// generate data for issue
	go func() {
		// every n minutes generate some duration that lasts m minutes
		ticker := time.NewTicker(1 * timeoutUnit)
		for {
			select {
			case <-ticker.C:
				fmt.Println("issue: creating")
				ie := buildIssuesEvent(repos, users)
				payload, err := json.Marshal(ie)
				if err != nil {
					fmt.Printf("issue: error json marshal: %s", err)
					panic(err)
				}

				resp, err := gh.PostEvent(payload, "issues", githubURL, client)
				if err != nil {
					panic(err)
				}
				resp.Body.Close()

				num := rand.Intn(10)
				fmt.Printf("issue: sleeping: %d %s\n", num, timeoutUnit.String())
				time.Sleep(time.Duration(num) * timeoutUnit)

				action := closed
				ie.Action = &action

				payload, err = json.Marshal(ie)
				if err != nil {
					fmt.Printf("issue: error json marshal: %s", err)
					panic(err)
				}

				fmt.Println("issue: closing")
				resp, err = gh.PostEvent(payload, "issues", githubURL, client)
				if err != nil {
					panic(err)
				}
				resp.Body.Close()
			}
		}
	}()

	// generate data for build
	buildStates := []string{"SUCCESS", "FAILURE"}
	go func() {
		// every n minutes generate some duration that lasts m minutes
		ticker := time.NewTicker(1 * timeoutUnit)
		for {
			select {
			case <-ticker.C:
				fmt.Println("build: creating")
				build := buildJenkinsEvent()
				payload, err := json.Marshal(build)
				if err != nil {
					fmt.Printf("build: error json marshal: %s", err)
					panic(err)
				}

				req, err := http.NewRequest(
					"POST",
					jenkinsURL.String()+"/",
					bytes.NewReader(payload),
				)
				if err != nil {
					panic(err)
				}
				resp, err := client.Do(req)
				if err != nil {
					panic(err)
				}
				resp.Body.Close()

				num := rand.Intn(10)
				fmt.Printf("build: sleeping: %d %s\n", num, timeoutUnit)
				time.Sleep(time.Duration(num) * timeoutUnit)

				build.Result = buildStates[rand.Intn(2)]

				payload, err = json.Marshal(build)
				if err != nil {
					fmt.Printf("build: error json marshal: %s", err)
					panic(err)
				}

				req, err = http.NewRequest(
					"POST",
					jenkinsURL.String()+"/",
					bytes.NewReader(payload),
				)
				if err != nil {
					panic(err)
				}
				fmt.Println("build: closing")
				resp, err = client.Do(req)
				if err != nil {
					panic(err)
				}
				resp.Body.Close()
			}
		}
	}()

	<-done
}
