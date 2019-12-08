<p align="center">
  <img src="docs/static/valuestream.png">
</p>

[ValueStream](https://medium.com/@dm03514/valuestream-devops-metrics-observing-delivery-across-multiple-systems-7ae76a6e8deb) provides a centralized view into key DevOps delivery metrics: Issues, Pull Requests, Builds and Deploys. ValueStream is a standalone HTTP service that listens for events (webhooks) from a variety of software platforms. ValueStream ships as a standalone service and using it is as easy as: 

- Starting valuestream and point it to an opentracing compliant backend (jaeger, lightstep, datadog, etc)
- Configuring github/jenkins webhook to point to value stream

ValueStream can help answer:
- What's the average/distribution time of open issues (per project, type, etc)?
- What's the average/distribution of build times (per project, type, etc)?
- What's the average/distribution of deploy times (per project, type, etc)?
- What's the average/distribution of pull request times (per project, repo, etc)?
- What's the deployment rate (per project, type, etc)?
- What's the deployment success ratio (per project, type, etc)?

# Quickstart (Sending Github Issue Data in 1 Minute!)

ValueStream can be started and submitting software metrics in minutes. [VIDEO QUICKSTART HERE](https://youtu.be/c7gD7WGqFxY) (video requires `docker-compose up -d`)

- Start ValueStream Docker Stack
```
$ docker-compose up -d
```
- Tail ValueStream
```
$ docker-compose logs -f valuestream 
Attaching to valuestream_valuestream_1
valuestream_1       | {"level":"info","msg":"initializing tracer: jaeger","time":"2019-07-19T20:12:56Z"}
valuestream_1       | 2019/07/19 20:12:56 Initializing logging reporter
valuestream_1       | 2019/07/19 20:12:56 Initializing logging reporter
valuestream_1       | {"level":"info","msg":"Starting Server: \":5000\"","time":"2019-07-19T20:12:56Z"}
valuestream_1       | {"buffer_percentage":0,"buffer_size":500,"curr_size":0,"level":"info","msg":"buffered_spans_state","name":"github","time":"2019-07-19T20:13:16Z"}
```
- Setup `ngrok` to access your local env 
```
$ ~/ngrok http 5000
```
- Point your github [webhook](https://developer.github.com/webhooks/) at `ngrok`

- Start tracking issues and pull requests!


# ValueStream In Action

ValueStream aggregates data from multiple different system and stores it in a standardize data model based on opentracing specification.  Having a standard data model allows for drilling down into individual integrations (think looking at just github pull request metrics) as well as providing cross service view (Tracing delivery time across multiple systems).  

## Devops Metrics

Valuestream is able to provide a cross system view into software development.  The dashboard below shows the average lead time across all issues from all systems (currently only Github issues are supported). The magic of having a standardized data model enables the view below to seamlessly work for github issues, jira issues, trello issues, or any other system that supports webhooks.  To drive this home consider a company that has code in both Github and Gitlab.  ValueStream can provide the average pull request duration across both github and gitlab, as well as the average across just github or just gitlab.  It even supports drilling down further by providing the average pull request duration by repo across both github and gitlab.

<p align="center">
  <img width="900px" src="docs/static/accelerate_dashboard.png">
</p>


## Traces

The real power of value stream comes from being able to tie together all the Delivery events (Issue, PRs, Builds & Deploys) from different sources.  When events are connected it is called a "Trace".  The image below shows the example of all steps required in order to produce a valuestream feature:

<p align="center">
  <img width="1448" alt="Screen Shot 2019-07-14 at 5 34 52 PM" src="https://user-images.githubusercontent.com/53025024/61565404-ed77e100-aa46-11e9-89f7-56ba7ba694ad.png">
</p>

To generate traces ValueStream leverages the OpenTracing ecosystem.  This defines a structured conventions to connecting data from multiple systems and provides mature client libraries and a rich infrastructure ecosystem.  In order to use get the most out of ValueStream it must be pointed at an opentracing stack.  [Jaeger](https://github.com/jaegertracing/jaeger) (by uber) is the most popular open source stack and has 8500+ stars on github.  Local development of ValueStream is done using jaeger. Any other opentracing compliant stack can be used (Datadog, Lightstep, etc).  ValueStream uses [LightStep](https://lightstep.com/) in production for development of ValueStream. 

Traces support drilling down into individual units or stages of work in order to see where time was spent in the delivery pipeline. This allows technical executives, managers, directors and VPs to debug software delivery in the same way an engineer debugs a distributed system, using datadriven hypothesis and measuring impact for each change. 

# Local Development

## Running Events Test Suite

- Start ValueStream
```
$ make start-valuestream-events-test
GO111MODULE=on \
        VS_LOG_LEVEL=debug \
        go run main.go -addr=:7778 -tracer=mock
INFO[0000] building tracer initializer for: "mock"       source="init.go:61"
{"level":"info","msg":"initializing source: \"github\"","time":"2019-12-08T07:11:12-05:00"}
{"level":"info","msg":"initializing source: \"gitlab\"","time":"2019-12-08T07:11:12-05:00"}
{"level":"info","msg":"initializing source: \"customhttp\"","time":"2019-12-08T07:11:12-05:00"}
{"level":"info","msg":"initializing source: \"jenkins\"","time":"2019-12-08T07:11:12-05:00"}
{"level":"info","msg":"initializing source: \"jira\"","time":"2019-12-08T07:11:12-05:00"}
{"level":"info","msg":"Starting Server: \":7778\"","time":"2019-12-08T07:11:12-05:00"}
```

- Execute Test Suite
```
$ make test-service-events

TEST_EVENTS_CUSTOM_HTTP_PATH="/customhttp" \
        TEST_EVENTS_JENKINS_PATH="/jenkins" \
        TEST_EVENTS_GITHUB_PATH="/github" \
        TEST_EVENTS_GITLAB_PATH="/gitlab" \
        TEST_EVENTS_JIRA_PATH="/jira" \
        TEST_EVENTS_URL=http://localhost:7778 \
        VS_LOG_LEVEL=DEBUG \
        go test \
                -run TestService \
                -tags=service \
                ./eventsources/... -count=1 -p 1
?       github.com/ImpactInsights/valuestream/eventsources      [no test files]
ok      github.com/ImpactInsights/valuestream/eventsources/github       0.103s
ok      github.com/ImpactInsights/valuestream/eventsources/gitlab       0.121s
ok      github.com/ImpactInsights/valuestream/eventsources/http 0.094s
ok      github.com/ImpactInsights/valuestream/eventsources/jenkins      0.098s
ok      github.com/ImpactInsights/valuestream/eventsources/jiracloud    0.115s
?       github.com/ImpactInsights/valuestream/eventsources/types        [no test files]
ok      github.com/ImpactInsights/valuestream/eventsources/webhooks     0.080s [no tests to run]
```

# Configuration
- Logging Level - Environmental Variable - `VS_LOG_LEVEL`
- Tracer Agent: CLI flag `-tracer=<<TRACER>>` which supports `logging|jaeger|lightstep`
-- Both jaeger and lightstep require additional configuration using their exposed environmental variables for their go client

# Roadmap
- OpenCensus Operational Metrics
- OpenCensus DevOps Metrics Exporter 
- Trace Strategy
- Trello
- Historical Data Import 



