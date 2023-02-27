# backplane-cli

backplane-cli is a CLI tool to interact with backplane api.

## Install

### Option 1: Use `go install`

You need to have `go` installed, if you want to use this method,
the minimal version required is `go 1.18`. You can check this with `go version`.

Then run `go install github.com/openshift/backplane-cli/cmd/ocm-backplane@{TAG_VERSION}`
where `TAG_VERSION` is the version you wish to install.

Example: `go install github.com/openshift/backplane-cli@0.0.8`

`go install` will fetch, build binary and install them to your $GOBIN if set, or $GOPATH/bin,
you should and move this binary onto your $PATH if desired.

### Option 2: Download binary

Download the latest binary file from the [release page](https://github.com/openshift/backplane-cli/releases).

For Linux, download `backplane-cli_<version>_Linux_x86_64`, rename it to `ocm-backplane` and put it to $PATH. For example:

```
$ sudo cp ocm-backplane /usr/bin/ocm-backplane
$ sudo chmod 0755 /usr/bin/ocm-backplane
```

For MacOS, download `ocm-backplane_darwin_amd64`, rename it to `ocm-backplane` and put it to $PATH. For example:

```
$ sudo cp ocm-backplane_darwin_amd64 /usr/local/bin/ocm-backplane
$ sudo chmod 0755 /usr/local/bin/ocm-backplane
```

To verify, you should see version output from backplane sub-command, like:

```
$ ocm backplane version
0.0.29
```

### Option 3: Build from source

First clone the repository somewhere in your $PATH. A common place would be within your $GOPATH.

Example:

```
$ mkdir $GOPATH/src/github.com/openshift
$ cd $GOPATH/src/github.com/openshift
$ git clone git@github.com/openshift/backplane-cli.git
```

```
$ make install
```

This command will build the backplane-cli binary, named `ocm-backplane`. This binary will be placed in $PATH. As the binary has prefix `ocm-`, it becomes a plugin of `ocm`, and can be invoked by `ocm backplane`.

For more information about ocm plugin, please refer https://github.com/openshift-online/ocm-cli#extend-ocm-with-plugins

### Setup bash/zsh prompt

To setup the PS1(prompt) for bash/zsh, please follow [these instructions](./docs/PS1-setup.md). Note that the "build with ocm-container" already has PS1 built-in.

## Usage

| Command                                                                     | Description                                                                              |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `ocm backplane login <CLUSTERID/EXTERNAL_ID/CLUSTER_NAME>`                  | Login to the target cluster                                                              |
| `ocm backplane logout <CLUSTERID/EXTERNAL_ID/CLUSTER_NAME>`                 | Logout of the target cluster                                                             |
| `ocm backplane console [flags]`                                             | Launch the OpenShift console of the current logged in cluster                            |
| `ocm backplane cloud console`                                               | Launch the current logged in cluster's cloud provider console                            |
| `ocm backplane cloud credentials [flags]`                                   | Retrieve a set of temporary cloud credentials for the cluster's cloud provider           |
| `ocm-backplane monitoring <prometheus/alertmanager/thanos/grafana> [flags]` | Launch the specified monitoring UI (Deprecated following v4.11)                          |
| `ocm-backplane project <project_name>`                                      | Manipulate the Kubeconfig and set the namespace of the current context to `project_name` |
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

### Example

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
  --> Console will be available at http://127.0.0.1:9000
  ```
- Follow the above link `http://127.0.0.1:9000` to access console.

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
- Or reach out [OWNERS](https://github.com/openshift/backplane-cli/-/blob/master/OWNERS)
