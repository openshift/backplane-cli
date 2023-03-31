# backplane-cli

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/backplane-cli)](https://goreportcard.com/report/github.com/openshift/backplane-cli)
[![codecov](https://codecov.io/gh/openshift/backplane-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift/backplane-cli)
[![GoDoc](https://godoc.org/github.com/openshift/backplane-cli?status.svg)](https://godoc.org/github.com/openshift/backplane-cli)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)

backplane-cli is a CLI tool to interact with [backplane api](https://github.com/openshift/backplane-api).

The onboarding documentation can be found on [The Source Wiki](https://source.redhat.com/groups/public/openshiftplatformsre/wiki/backplane_user_documentation).

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

Please refer to the [Source Wiki](https://source.redhat.com/groups/public/openshiftplatformsre/wiki/backplane_user_documentation) for more details.

## Setup bash/zsh prompt

To setup the PS1(prompt) for bash/zsh, please follow [these instructions](https://github.com/openshift/backplane-cli/blob/main/docs/PS1-setup.md).

## Usage

| Command                                                                     | Description                                                                              |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `ocm backplane login <CLUSTERID/EXTERNAL_ID/CLUSTER_NAME>`                  | Login to the target cluster                                                              |
| `ocm backplane logout <CLUSTERID/EXTERNAL_ID/CLUSTER_NAME>`                 | Logout from the target cluster                                                             |
| `ocm backplane console [flags]`                                             | Launch the OpenShift console of the current logged in cluster                            |
| `ocm backplane cloud console`                                               | Launch the current logged in cluster's cloud provider console                            |
| `ocm backplane cloud credentials [flags]`                                   | Retrieve a set of temporary cloud credentials for the cluster's cloud provider           |
| `ocm backplane elevate <reason> -- <command>`                               | Elevate privileges to backplane-cluster-admin and add a reason to the api request        |
| `ocm-backplane monitoring <prometheus/alertmanager/thanos/grafana> [flags]` | Launch the specified monitoring UI (Deprecated following v4.11)                          |
| `ocm-backplane script describe <script> [flags]`                            | Describe the given backplane script                                                      |
| `ocm-backplane script list [flags]`                                         | List available backplane scripts                                                         |
| `ocm-backplane status`                                                      | Print essential cluster info                                                             |
| `ocm-backplane managedJob create <script> [flags]`                          | Create a backplane managed job resource                                                  |
| `ocm-backplane managedJob get <job_name> [flags]`                           | Retrieve a backplane managed job resource                                                |
| `ocm-backplane managedJob list [flags]`                                     | Retrieve a list of backplane managed job resources                                       |
| `ocm-backplane managedJob logs <job_name> [flags]`                          | Retrieve logs of the specified managed job resource                                      |
| `ocm-backplane managedJob delete <job_name> [flags]`                        | Delete the specified managed job resource                                                |
| `ocm-backplane testJob create <script> [flags]`                             | Create a backplane test job on a non-production cluster for testing                      |
| `ocm-backplane testJob get <job_name> [flags]`                              | Retrieve a backplane test job resource                                                   |
| `ocm-backplane testJob list [flags]`                                        | Retrieve a list of backplane test job resources                                          |
| `ocm-backplane testJob logs <job_name> [flags]`                             | Retrieve logs of the specified test job resource                                         |
| `ocm-backplane upgrade`                                                     | Upgrade backplane-cli to the latest version                                              |
| `ocm-backplane version`                                                     | Display the installed backplane-cli version                                              |

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
