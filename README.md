# backplane-cli

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/backplane-cli)](https://goreportcard.com/report/github.com/openshift/backplane-cli)
[![codecov](https://codecov.io/gh/openshift/backplane-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/openshift/backplane-cli)
[![GoDoc](https://godoc.org/github.com/openshift/backplane-cli?status.svg)](https://godoc.org/github.com/openshift/backplane-cli)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)

backplane-cli is a CLI tool to interact with [backplane api](https://github.com/openshift/backplane-api).

## Installation

Go should be installed in your local system with version 1.19 

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

**Security Note:** This configuration file may store sensitive information such as PagerDuty API keys or JIRA tokens. It is recommended to ensure its permissions are restrictive (e.g., `chmod 600 $HOME/.config/backplane/config.json`) to protect this data.

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
| `ocm backplane cloud ssm --node <node-name>`                                | Start an aws ssm session for an HCP cluster                                              |
| `ocm backplane elevate <reason> -- <command>`                               | Elevate privileges to backplane-cluster-admin and add a reason to the api request, this reason will be stored for 20min for future usage        |
| `ocm backplane monitoring <prometheus/alertmanager/thanos/grafana> [flags]` | Launch the specified monitoring UI (Deprecated following v4.11 for cluster monitoring stack)|
| `ocm backplane script describe <script> [flags]`                            | Describe the given backplane script                                                      |
| `ocm backplane script list [flags]`                                         | List available backplane scripts |
| `ocm backplane session [flags]`                                             | Create a new session and log into the cluster                                            |
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
| `ocm backplane healthcheck`                                                 | Check the VPN and Proxy connectivity on the host network when experiencing isssues accessing the backplane API|

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
  system:serviceaccount:default:1234567
  ```
  
- To login to the Management cluster for HyperShift (or) the managing Hive shard of normal OSD/ROSA cluster
  ```
  $ ocm backplane login <cluster> --manager
  ```
- To login to the Service Cluster of a HyperShift hosted cluster or the Management Cluster
  ```
  $ ocm backplane login <cluster> --service
  ```
### Get cluster information after login

- Login to the target cluster via backplane and add `--cluster-info` flag
 ```
  $ ocm backplane cluster login <cluster> --cluster-info
 ```
- Set a `"display-cluster-info": true` flag in the backplane config for cluster info to be auto printed. 
 > Note: `"display-cluster-info": true` has to be set as a `boolean` value.
  
 ```
  {
    "proxy-url": "your-proxy-url",
    "display-cluster-info": true
  }
 ```
### Login to multiple clusters 

Logging into multiple clusters via different terminal instances.
- How to log into the first cluster

  ```
  $ ocm backplane login <cluster-id-1> --multi
  $ export KUBECONFIG= <cluster-id-1-kube-config-path>
  ```

  you can also directly run

  ```
  $ source <(ocm backplane login <cluster-id-1> --multi)
  ```

- How to log into the second cluster

  ```
  $ ocm backplane login <cluster-id-2> --multi
  $ export KUBECONFIG= <cluster-id-2-kube-config-path>
  ```

### Login through PagerDuty incident link or ID

- [Generate a User Token REST API Key](https://support.pagerduty.com/docs/api-access-keys#generate-a-user-token-rest-api-key) and save it into backplane config file.
  ```
  $ ocm backplane config set pd-key <api-key>
  ```
  Replace `<api-key>` with the actual User Token REST API Key obtained from PagerDuty.

- To log in using the PagerDuty incident link, use the following command:
  ```
  $ ocm backplane login --pd https://{your-pd-domain}.pagerduty.com/incidents/<incident-id>
  ```
  Replace `<incident-id>` with the specific incident ID you want to access.

- Alternatively, if you have the incident ID, you can use the following command:
  ```
  $ ocm backplane login --pd <incident-id>
  ```
  Replace `<incident-id>` with the specific incident ID you want to access.

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

## SSM Session
Now you can directly start an AWS SSM session in your terminal using a single command for the HCP clusters without logging into their cloud consoles. It will start an AWS session directly in your terminal where you can debug into the worker node for the HCP cluster and carry out further operations.
- Before using ssm command check if Session Manager plugin has been properly set up in your device. Follow this official AWS [documentation](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-prerequisites.html) for further information on setting up AWS SSM. And for installing SSM plugin directly on your device follow this [documentation](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html). Also check AWS CLI version and SSM version and ensure that those are in required versions. Update if required.
- Login to the management cluster for the target HCP cluster 
```
$ ocm backplane login <cluster> --manager
```
- Run the following command by providing the correct worker node name where you want to start a SSM Session
```
$ ocm backplane cloud ssm --node <node-name>` 
```
- It will show something like this:
```
$ ocm-backplane cloud ssm --node ip-xx-x-xxx-xxx.xxxxxx.compute.internal 

Starting session with SessionId: e4abf0bf76199710d76b3ecaa2c6f4ae-929p9nkk8yta7upy8el4ylidq4
sh-5.1$
```

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

Backplane session setup following environment variables.
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
## Backplane elevate
If you need to run some oc command(s) with elevation using backplane-cluster-admin user, you can use the elevate command for this.

Backplane elevate takes as first positional argument the reason for this elevation. If the first argument is an empty string, then it will be considered as an empty reason, but you cannot just skip the reason argument if you provide also other positional argument(s).
If you want to not provide an empty string as reason, you can use the -n/--no-reason option and oc command will start at first positional argument.

The elevate command requires a none empty reason for the elevation. When a reason is provided it will be used for future usage, in order you do not have to provide a reason for each elevation commands. The reasons are stored in the kubeconfig context, so it is valid only for the cluster for which it has been provided. When a reason is created/used, the last used reason timestamp is updated in the context, and the reason will be kept for 20min after its last usage, in order to avoid bad usage.

When you use the elevate command with an empty reason, it will look if a non expired reason is stored in the current context for this server, and if there is one it will use it. If there is no reason stored in current context, then if the stdin and stderr are not redirected to pipe or file, a prompt will be done to ask for the reason.

### Run an elevate command with reason
```
$ ocm-backplane evate 'OHSS-xxxxxx' -- get secret xxx
```
The provided reason will be used for elevation, but also stored for future elevation on this cluster.
If a reason was already stored in the current_context, then this provided reason will be added to it.

### Run an elevate command with empty reason
If you run the elevate command with an empty reason for the first time (or after the expiration), then you will be prompt for the reason if possible
```
$ ocm-backplane elevate '' -- get secret xxx
Please enter a reason for elevation, it will be stored in current context for 20 minutes : <here you can enter your reason>
```
or 
```
$ ocm-backplane elevate -n -- get secret xxx
Please enter a reason for elevation, it will be stored in current context for 20 minutes : <here you can enter your reason>
```
If then you rerun an elevate command, for the same cluster, before the expiration delay, no prompt will be done and previous reason will be used for elevation.

### Run elevate without command
You can initialize the reson context for a cluster without running a command, then the reason will be used for future commands
```
$ ocm-backplane elevate 'OHSS-xxxxxx'
```
or you can not provide the reason and will be prompt for it if needed
```
$ ocm-backplane elevate
Please enter a reason for elevation, it will be stored in current context for 20 minutes : <here you can enter your reason>
```

### Run elevate without (stored) reason and without valid prompt

If a prompt is required but that stdin and/or stderr are redirected to file or output, then an error will be generated.
```
$ cat patch.json | ocm-backplane elevate -n -- patch -f -
ERRO[0000] please enter a reason for elevation
$ ocm-backplane elevate -n -- get secret xxx 2> error.txt
ERRO[0000] please enter a reason for elevation
```
In order to avoid those errors, you can either run the the elevate without command before or provide a none empty reason.

No issue if only stdout is redirected.
```
$ ocm-backplane elevate -n -- get secret xxx | grep xxx
Please enter a reason for elevation, it will be stored in current context for 20 minutes : <here you can enter your reason>
```
## Backplane healthcheck
The backplane health check can be used to verify VPN and proxy connectivity on the host network as a troubleshooting approach when experiencing issues accessing the backplane API.

### Pre-settings
The end-user needs to set the VPN and Proxy check-endpoints in the local backplane configuration first:
```
cat ~/.config/backplane/config.json 
{
    "proxy-url": ["http://proxy1.example.com:3128", "http://proxy2.example.com:3128"],
    "vpn-check-endpoint": "http://your-vpn-endpoint.example.com",
    "proxy-check-endpoint": "http://your-proxy-endpoint.example.com"
}
```
- `vpn-check-endpoint:` To specify this test endpoint to check if it can be accessed with the currently connected VPN.
- `proxy-check-endpoint:` To specify this test endpoint to check if it can be accssed with the currently working proxy.

**NOTE:** The `vpn-check-endpoint` and `proxy-check-endpoint` mentioned above are just examples, the end-user can customize them as needed.

### How to use it
- Running healthcheck by default
```
./ocm-backplane healthcheck
Checking VPN connectivity...
VPN connectivity check passed!

Checking proxy connectivity...
Getting the working proxy URL ['http://proxy1.example.com:3128'] from local backplane configuration.
Testing connectivity to the pre-defined test endpoint ['https://your-proxy-endpoint.example.com'] with the proxy.
Proxy connectivity check passed!

Checking backplane API connectivity...
Successfully connected to the backplane API!
Backplane API connectivity check passed!
```
- Specify the healthcheck flags to run `vpn` or `proxy` check only
```
./ocm-backplane healthcheck --vpn
Checking VPN connectivity...
VPN connectivity check passed!

./ocm-backplane healthcheck --proxy
Checking proxy connectivity...
Proxy connectivity check passed!
```

**Note:** VPN connection check is a pre-requisite (The example below demonstrates checking proxy connectivity with VPN disconnected.)
```
./ocm-backplane healthcheck --proxy
WARN[0000] No VPN interfaces found: [tun tap ppp wg]    
VPN connectivity check failed: No VPN interfaces found: [tun tap ppp wg]
Note: Proxy connectivity check requires VPN to be connected. Please ensure VPN is connected and try again.
```

## Promotion/Release cycle of backplane CLI
Backplane CLI has a default release cycle of every 2 weeks 

In case of you have changes that have immediate impact and would need an immediate promotion, please reach out to:

Backplane team (alias : @backplane-team) in #sd-ims-backplane slack channel 

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

## Vulnerability Scanning with `govulncheck`

As part of our continuous integration (CI) process, we've incorporated `govulncheck` to identify known vulnerabilities in the `backplane-cli` codebase.
In the CI environment (specifically the `ci/prow/scan-optional` test), vulnerability reports may be available as artifacts, potentially named `build-log.txt` or similar, within the test's artifact storage. This is nested within
the `artifacts/test/` directory of the `ci/prow/scan-optional` test. To retrieve the report:
- Click on `Details` next to `ci/prow/scan-optional` in a specific PR.
- Click `Artifacts` at the top-right corner of the page.
- Navigate to `artifacts/test/` to view the `build-log.txt` (or similarly named file) containing vulnerability information.

While some detected vulnerabilities might be non-blocking at the moment, they are still reported. We encourage both users and developers to thoroughly
review these reports. If any Go packages are flagged, consider updating them to their fixed versions.

To manually execute a vulnerability scan locally, run the following command:
```
make scan
```

**Note on Local Scans:** Running `govulncheck` (e.g., via `make scan`) locally can be sensitive to the Go version and your development environment. If you encounter errors, ensure your Go version aligns with the one used in CI, or refer to the "Fixing Go Version Compatibility Issues" section. The `make scan` command currently prints output to standard output; it does not generate a `build-log.txt` file locally.

## Fixing Go Version Compatibility Issues
When build failures occur due to Go version mismatches, follow these steps:

- Create Release Repository PR
Open a PR against the openshift/release repository to update the CI configuration to the latest Go version.
Example: PR [#62885](https://github.com/openshift/release/pull/62885): Bump to Go 1.21

- Merge Release PR
Wait for the release repository PR to be merged by the CI bot.

- Update Backplane CLI
Update the Go version in the Backplane CLI Dockerfile and verify CI builds:

```
FROM golang:1.21  # Update version to match release PR
```
Example Implementation: PR [#636](https://github.com/openshift/backplane-cli/pull/636): OSD-28717 Fix build failures
Update the dockerfile of backplane-cli with the latest go version and check if build passes.
Check for any issues while updating the dockerfile and start a thread in #sd-ims-backplane channel to mitigate this issue.