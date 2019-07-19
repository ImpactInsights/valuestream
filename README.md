<p align="center">
  <img src="docs/static/valuestream.png">
</p>

DevOps Accelerate Metrics.  One Service. One View. All your tools.

[ValueStream](https://medium.com/@dm03514/valuestream-devops-metrics-observing-delivery-across-multiple-systems-7ae76a6e8deb) provides a centralized view into key DevOps delivery metrics. If you’ve ever wondered how long tickets are open, or how long deployments take or the success rate of deployments, or the duration of pull requests, ValueStream can help you.  ValueStream is a standalone HTTP service that listens for events (webhooks) from Github and Jenkins. ValueStream leverages opentracing as a metric primitive. Because of this, valuestream ships as a standalone service and using it is as easy as: starting valuestream and point it to an opentracing compliant backend (jaeger, lightstep, datadog, etc), and then configuring github/jenkins webhook to point to value stream.

ValueStream can help answer:
- What's the average/distribution time of open issues (per project, type, etc)?
- What's the average/distribution of build times (per project, type, etc)?
- What's the average/distribution of deploy times (per project, type, etc)?
- What's the average/distribution of pull request times (per project, repo, etc)?
- What's the deployment rate (per project, type, etc)?
- What's the deployment success ratio (per project, type, etc)?

# ValueStream In Action

ValueStream aggregates data from multiple different system and stores it in a standardize data model based on opentracing specification.  Having a standard data model allows for drilling down into individual integrations (think looking at just github pull request metrics) as well as providing cross service view (Tracing delivery time across multiple systems).  An example of each of these is shown below.

## Devops Metrics

Valuestream is able to provide a cross system view into software development.  The dashboard below shows the average lead time across all issues from all systems (currently only Github issues are supported). The magic of having a standardized data model enables the view below to seamlessly work for github issues, jira issues, trello issues, or any other system that supports webhooks.  To drive this home consider a company that has code in both Github and Gitlab.  ValueStream can provide the average pull request duration across both github and gitlab, as well as the average across just github or just gitlab.  It even supports drilling down further by providing the average pull request duration by repo across both github and gitlab.

<p align="center">
  <img width="900px" src="docs/static/accelerate_dashboard.png">
</p>

## Traces



# Quickstart
- Start ValueStream Docker
- Configure Github Webhook


# Features
- Track Duration of github issues/pull requests
- Track Duration of jenkins builds
- Tie together 
- Opentracing ecosystem
- Unififed view of software metrics across multiple systems

# Architecture


# Traces
In order to get the full value from ValueStream metrics need to be [connected in some way](https://opentracing.io/specification/#the-opentracing-data-model).  "Trace Propagation" is ValueStreams term for connencting these.  The supported relationships are visualized as:

<p align="center">
  <img width="500px" src="docs/static/span_relationships.png">
</p>

The following table shows the currently supported actions, which service they originate from, and how they are identified within traces. The chart below shows each action and how it's referenced by child actions.  The first is a github issue.  It can be referenced by a Pull Request or a build using the `TRACE_ID` of the form `vstrace-github-{{ REPO_NAME }}-{{ ISSUE_NUMBER }}`.  

| Integration Name | Type |  Referenced By | TRACE_ID | Internal Prefix |
|:-----:|--------|--------| ----------| ----|
|   Github    | Issue     | ISSUE_NUMBER | `vstrace-github-{{ REPO_NAME }}-{{ ISSUE_NUMBER }}` | `Traces.ISSUE-vstrace-github-{{ REPO_NAME }}-{{ ISSUE_NUMBER }}` |
|   Github    | Pull Request      | BRANCH |  `git checkout -b  {{ BRANCH }}` | `Spans.{{ PULL_REQUEST_ID }}` |
|   Jenkins    | Pull Request      | N/A | N/A  | `Spans.{{ PULL_REQUEST_ID }}` |


The next chart shows how children nodes are able to reference their parent nodes.  The children node relationships are (PullRequest -> Issue), (Build -> PullRequest), and (Build, Issue).  Listed below shows each pair is able to reference the other in order to form full traces:

## Referencing Nodes

### PullRequest -> Issue
This can be used to track all code associated with a given ticket/issue.

Produces:

<p align="center">
  <img width="900" src="https://user-images.githubusercontent.com/321963/61335859-02f9ca80-a7fd-11e9-9b58-bed7266a2f14.png">
</p>

The pull request branch needs to include the Issue `TRACE_ID` somewhere:

```
$ git checkout -b feature/vstrace-github-{{ REPO_NAME }}-{{ ISSUE_NUMBER }}/my-issue
```
<p align="center">
  <img width="900" alt="Screen Shot 2019-07-11 at 4 50 36 PM" src="https://user-images.githubusercontent.com/321963/61084593-4b2f7c00-a3fc-11e9-8570-a5e9e2ee6ef2.png">
</p>

To reference the issue above from a pull request the branch name must include:

```
vstrace-github-valuestream-27
```

## Build -> PullRequest
This relationship helps to capture all CI builds associated with a given pull request. The 

<p align="center">
  <img width="900px" src="docs/static/lightstep_pr_build_1.png">
</p>


## Build -> Issue








#### Pull Request

##### Branch Name
Branch names can reference a `TRACE_ID`

<img width="600" alt="Screen Shot 2019-07-11 at 4 57 15 PM" src="https://user-images.githubusercontent.com/321963/61084932-12dc6d80-a3fd-11e9-8af8-fb2d9fb184b9.png">

### Jenkins

#### Build

##### Parameter
Build Parameters can reference any Parent Span of type `ISSUE` by it's public `TRACE_ID`.

<img width="1377" alt="Screen Shot 2019-07-11 at 7 09 39 PM" src="https://user-images.githubusercontent.com/321963/61091311-b0d93380-a40f-11e9-82b8-bc1123165d71.png">

##### SCM Integration

SCM triggered builds (ie github webhooks) can reference a Pull Request as parent through its branch name.

