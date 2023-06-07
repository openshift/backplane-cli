# backplane-cli

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/backplane-cli)](https://goreportcard.com/report/github.com/openshift/backplane-cli)
[![codecov](https://codecov.io/gh/openshift/backplane-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift/backplane-cli)
[![GoDoc](https://godoc.org/github.com/openshift/backplane-cli?status.svg)](https://godoc.org/github.com/openshift/backplane-cli)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)

backplane-cli is a CLI tool to interact with [backplane api](https://github.com/openshift/backplane-api).

## Installation

### Option 1: Download from release

Download the latest binary from the GitHub [releases](https://github.com/openshift/backplane-cli/releases) page.

For example, to download the binary on Linux:

```
$ wget https://github.com/openshift/backplane-cli/releases/download/v0.1.1/ocm-backplane_0.1.1_Linux_x86_64.tar.gz
$ tar -xvzf ocm-backplane_0.1.1_Linux_x86_64.tar.gz
$ chmod +x ocm-backplane
$ mv ocm-backplane $GOBIN
```

### Option 2: Build from source

First clone the repository somewhere in your `$PATH`. A common place would be within your `$GOPATH`.

Example:

```
$ mkdir $GOPATH/src/github.com/openshift
$ cd $GOPATH/src/github.com/openshift
$ git clone git@github.com/openshift/backplane-cli.git
```

```
$ make build
```

This command will build the backplane-cli binary, named `ocm-backplane`. This binary will be placed in $PATH.
As the binary has prefix `ocm-`, it becomes a plugin of `ocm`, and can be invoked by `ocm backplane`.

For more information about ocm plugins, please refer https://github.com/openshift-online/ocm-cli#extend-ocm-with-plugins

## Configuration

The configuration file of backplane-cli is expected to be located at `$HOME/.config/backplane/config.json`.

## Setup bash/zsh prompt

To setup the PS1(prompt) for bash/zsh, please follow [these instructions](https://github.com/openshift/backplane-cli/blob/main/docs/PS1-setup.md).

## Usage

| Command                                                                     | Description                                                                              |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `ocm backplane login <CLUSTERID/EXTERNAL_ID/CLUSTER_NAME>`                  | Login to the target cluster                                                              |
| `ocm backplane logout <CLUSTERID/EXTERNAL_ID/CLUSTER_NAME>`                 | Logout from the target cluster                                                           |
| `ocm backplane config get [flags]`                                          | Retrieve Backplane CLI configuration variables                                           |
| `ocm backplane config set [flags]`                                          | Set Backplane CLI configuration variables                                                |
| `ocm backplane console [flags]`                                             | Launch the OpenShift console of the current logged in cluster                            |
| `ocm backplane cloud console`                                               | Launch the current logged in cluster's cloud provider console                            |
| `ocm backplane cloud credentials [flags]`                                   | Retrieve a set of temporary cloud credentials for the cluster's cloud provider           |
| `ocm backplane elevate <reason> -- <command>`                               | Elevate privileges to backplane-cluster-admin and add a reason to the api request        |
| `ocm backplane monitoring <prometheus/alertmanager/thanos/grafana> [flags]` | Launch the specified monitoring UI (Deprecated following v4.11 for cluster monitoring  stack)                          |
| `ocm backplane script describe <script> [flags]`                            | Describe the given backplane script                                                      |
| `ocm backplane script list [flags]`                                         | List available backplane scripts                                                         |
| `ocm backplane status`                                                      | Print essential cluster info                                                             |
| `ocm backplane managedJob create <script> [flags]`                          | Create a backplane managed job resource                                                  |
| `ocm backplane managedJob get <job_name> [flags]`                           | Retrieve a backplane managed job resource                                                |
| `ocm backplane managedJob list [flags]`                                     | Retrieve a list of backplane managed job resources                                       |
| `ocm backplane managedJob logs <job_name> [flags]`                          | Retrieve logs of the specified managed job resource                                      |
| `ocm backplane managedJob delete <job_name> [flags]`                        | Delete the specified managed job resource                                                |
| `ocm backplane testJob create <script> [flags]`                             | Create a backplane test managed job on a non-production cluster for testing. To use with bash libraries, make sure the libraries are in the scripts directory in the format `source /managed-scripts/<path-from-managed-scripts-scripts-dir>`                      |
| `ocm backplane testJob get <job_name> [flags]`                              | Retrieve a backplane test job resource                                                   |
| `ocm backplane testJob list [flags]`                                        | Retrieve a list of backplane test job resources                                          |
| `ocm backplane testJob logs <job_name> [flags]`                             | Retrieve logs of the specified test job resource                                         |
| `ocm backplane upgrade`                                                     | Upgrade backplane-cli to the latest version                                              |
| `ocm backplane version`                                                     | Display the installed backplane-cli version                                              |

## Login

#### Example

In this example, we will login to a cluster with id `123456abcdef` in production environment, and we have the OCM client environment setup [like this](https://github.com/openshift-online/ocm-cli#log-in).
- Run backplane login in another terminal.

  ```
  $ ocm backplane login <cluster>
  ```

- Run `oc` command to access the target cluster.
  ```
  $ oc whoami
  system:serviceaccount:openshift-backplane-srep:1234567
  ```
  
- To login to the Management cluster for HyperShift (or) the managing Hive shard of normal OSD/ROSA cluster
  ```
  $ ocm backplane login <cluster> --manager
  ```
- To login to the Service Cluster of a HyperShift hosted cluster or the Management Cluster
  ```
  $ ocm backplane login <cluster> --service
  ```

### Login to multiple clusters 

Logging into multiple clusters via different terminal instances.
- How to log into the first cluster

  ```
  $ ocm backplane login <cluster-id-1> --multi
  $ export KUBECONFIG= <cluster-id-1-kube-config-path>
  ```

- How to log into the second cluster

  ```
  $ ocm backplane login <cluster-id-2> --multi
  $ export KUBECONFIG= <cluster-id-2-kube-config-path>
  ```

## Console

- Login to the target cluster via backplane as the above.
- Run the below command and it will launch the console of the current logged in cluster.
  ```
  $ ocm backplane console
  --> Console will be available at http://127.0.x.x:xxxx
  ```
- Follow the above link `http://127.0.x.x:xxxx` to access console.

  #### Open in browser

  You can directly open the console in browser by adding flag -b or setting environment variable `BACKPLANE_DEFAULT_OPEN_BROWSER=true`. Example,

  When running this command, it will open the console in the browser automatically.

  ```
  $ ocm backplane console -b
  ```

  Or set the environment variable

  ```
  $ export BACKPLANE_DEFAULT_OPEN_BROWSER=true

  $ ocm backplane console
  ```

  Optionally, you can also load the enabled console plugin
  ```
  $ ocm backplane console -plugins
  ```
  > Note: Load the console plugin from backplane-cli is not sufficient to access the console plugin,
  backplane-api to expose the console plugin service explicitly is needed.

## Cloud Console

- Login to the target cluster via backplane as the above.
- Run the below command and it will launch the cloud console of the current logged in cluster.
  ```
  $ ocm backplane cloud console
  Console Link:
  Link: https://xxxxx
  ```
- Follow the above link to access the console.

  #### Open in browser

  You can directly open the console in the browser by adding flag `-b` or setting the environment variable `BACKPLANE_DEFAULT_OPEN_BROWSER=true`.
  
  When running this command, it will open the console in the browser automatically.

  ```
  $ ocm backplane cloud console -b
  ```

  Or set the environment variable

  ```
  $ export BACKPLANE_DEFAULT_OPEN_BROWSER=true
  $ ocm backplane cloud console
  `
## Monitoring
Monitoring command can be used to launch the specified monitoring UI.

Run this command from within a cluster :

```
ocm backplane monitoring <prometheus/alertmanager/thanos/grafana> [flags]
```

>Note: Following version 4.11, Prometheus, AlertManager and Grafana monitoring UIs are deprecated for openshift-monitoring stack, please use 'ocm backplane console' and use the observe tab for the same. Other monitoring stacks remain unaffected.
## Backplane Session 
Backplane session command will create an isolated environment to interact with a cluster in its own directory. 
The default location for this is ~/backplane. 

The default session save path can be configured via the backplane config file. 
```
{
   "url": "your-bp-url"
   "proxy-url": "your-proxy-url"
   "session-dir":"your-session-dir"
}
```
### How to create new session?
The following command will create a new session and log in to the cluster.
```
## with intractive session name 
ocm backplane session <session-name> -c <cluster-id>

## only with cluster id
ocm backplane session <cluster-id> 
```

Backplane session keeps the session history commands in <your-path>/session-name/.history file.

```
[ <session-name> (<cluster-info-PS1>)]$ history 
    1  2023-05-08 15:06:05 oc get nodes
    2  2023-05-08 15:06:13 oc get co
    3  2023-05-08 15:06:40 history 
```

Backpalane session setup following environment variables.
```
HISTFILE    = <your-session-path>/<session-name>/.history
PATH        = <your-os-path>
KUBECONFIG  = <your-session-path>/<session-name>/<cluster-id>/config
CLUSTERID   = <cluster-id>
CLUSTERNAME = <cluster-name>
```

### How to delete the session?
Folowing command delete the session
```
ocm backplane session --delete <session-name>
```


## Debugging issues

To help diagnose any issues, you can modify the default verbosity of the logger. Use `-v` for `info` level or explicitly setting the logging level by using `--verbosity=debug` flag.

For further information on logging levels refer to the in-built help.

```
$ ocm backplane help
```

## How backplane-cli works
See [design.md](docs/design.md).

Please help us to improve. To contact the backplane team:

- @backplane-team in slack channel #sd-ims-backplane (CoreOS workspace)
- Or reach out [OWNERS](https://github.com/openshift/backplane-cli/blob/main/OWNERS)
