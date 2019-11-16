PKGS = $(shell go list ./... | grep -v /vendor/)

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

test-service-github-jenkins:
	GO111MODULE=on \
	VALUESTREAM_URL=http://localhost:5000 \
		go test -run TestGithubJenkinsTrace ./traces/trace_service_test.go -v -count=1

test-service-github-ci-build-jenkins:
	GO111MODULE=on \
	VALUESTREAM_URL=http://localhost:5000 \
		go test -run TestGithubJenkinsPRBuildJenkinsDeployTrace ./traces/trace_service_test.go -v -count=1


.PHONY: test-unit start-stack fmt