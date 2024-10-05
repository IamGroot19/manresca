# Introduction

### What is it?

- Dealing with helm charts, especially a complicated one which has 20+ different subcharts tacked on to it, is complicated. What's even more complicated is to try to get an estimate of total resources the chart is going to need before you deploy it. 

- So, I wrote up a Golang based CLI `manresca` (manifests resources calculator) to do the job for me instead of havig to eyeball yaml files

### Why? 

(why you might want to have a rough idea of total resources your app is going to take up?)

- You have an on-prem or self-managed K8s cluster. So, it's important to have a decent estimate for amount of resources (cpu, memory, storage) coz you have  finite servers on your DC / colo
- Maybe you want to deploy this app on a dedicated nodepool and want a rough idea cpu:memory ratio to determine optimal instance type & no. of required instances
- You might say, "But..but..but... I have a managed kubernetes cluster with ClusterAutoscaler / karpenter enabled and I have a wide variety of instance types in my nodepool, so I am sure kubernetes will take care of scheduling my pods accordingly". Even then, it wouldnt hurt to get a rough idea of required resources, coz somewhere you might have added an extra zero accidentally, which is preventable.

### When/Where can you use it?
- CI Pipelines would be the best place to use this, assuming a standard SRE/Platform team (which means Gitops paradigm is religously followed). This would ensure you dont get suprised after deploying the chart.

- But you can also use it as a sanity check before creating Merge Requests on your local laptop

### How to use it?

Currently, the project isn't dockerised nor does it have github actions based releases. (I plan to set it up as soon as I get some bandwidth though). Till then, pls clone the repo and build from source. 
```
$ go build -o manresca
```

Usage it as a CLI:
```
$ ./manresca help

Usage:
  manresca [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  estimate    A brief description of your command
  help        Help about any command

Flags:
  -h, --help     help for manresca
  -t, --toggle   Help message for toggle

Use "manresca [command] --help" for more information about a command.


# Sample Usage Command
$ ./manresca estimate -f examples/combined_manifests.yaml  --verbosity 0
```

## Features 

### Current Features
- CLI Tool
- Can parse following Kubernetes objects: `Pod`, `Deployment`, `Statefulset`, `Job`, `CronJob`
- Only helm rendered manifest(s) are supported  (i.e the output of `helm template --debug <chart-path> -f <valuesfile> -f <valuesfile>...`)
- A `verbosity` flag which allows you to see different levels of info:
  - V=0 BASIC (just a summary of Req & limits for each workload)
  - V=1: Req/Lim multiplied by no. of replicas accounting for Horizontal Pod Autoscalers. The total usage is also calculated at the bottom

### Future features / Improvements

Low Hanging
- Export data as CSV File Formats

(Potential features) Need to research / read more to figure out feasibility
- Extend the calculations to storage
- Extend the calculations for CRDs/CRs. For eg, calculate resources when a MariaDB CR (belonging to MariaDB operator) is created.
- Take KEDA Scalers into account for calculating upper & lower bounds (overlaps with previous feature related to CRDs)


===========

That's it. Pls do reachout if you have any feedbacks (code-wise / feature-wise / performance wise). Thanks for your time!

