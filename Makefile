PKGS = $(shell go list ./... | grep -v /vendor/)
TEST_EVENTS_CUSTOM_HTTP_PATH ?= "/customhttp"
TEST_EVENTS_JENKINS_PATH ?= "/jenkins"
TEST_EVENTS_GITHUB_PATH ?= "/github"
TEST_EVENTS_GITLAB_PATH ?= "/gitlab"

test-unit:
	GO111MODULE=on go test -tags=unit -coverprofile=coverage.out $(PKGS)

build:
	GO111MODULE=on go build ./...

fmt:
	go fmt $(PKGS)

start-stack:
	cd test/jaeger-stack && docker-compose down && docker-compose up

start-valuestream-local:
	GO111MODULE=on \
	JAEGER_REPORTER_LOG_SPANS=1 \
	JAEGER_SAMPLER_TYPE=const \
	JAEGER_SAMPLER_PARAM=1 \
	VS_LOG_LEVEL=debug \
	go run main.go -addr=:5000 -tracer=jaeger

start-valuestream-service-test:
	GO111MODULE=on \
	VS_LOG_LEVEL=debug \
	go run main.go -addr=:7778 -tracer=mock

test-service-github-jenkins:
	GO111MODULE=on \
	VALUESTREAM_URL=http://localhost:5000 \
		go test -run TestGithubJenkinsTrace ./traces/trace_service_test.go -v -count=1

test-service-github-ci-build-jenkins:
	GO111MODULE=on \
	VALUESTREAM_URL=http://localhost:5000 \
		go test -run TestGithubJenkinsPRBuildJenkinsDeployTrace ./traces/trace_service_test.go -v -count=1

test-service-events:
	TEST_EVENTS_CUSTOM_HTTP_PATH=$(TEST_EVENTS_CUSTOM_HTTP_PATH) \
	TEST_EVENTS_JENKINS_PATH=$(TEST_EVENTS_JENKINS_PATH) \
	TEST_EVENTS_GITHUB_PATH=$(TEST_EVENTS_GITHUB_PATH) \
	TEST_EVENTS_GITLAB_PATH=$(TEST_EVENTS_GITLAB_PATH) \
	TEST_EVENTS_URL=http://localhost:7778 \
	go test \
		-run TestService \
		-tags=service \
		./eventsources/... -count=1

.PHONY: test-unit start-stack fmt