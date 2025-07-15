# Contributing to backplane-cli

## User Experience Guidelines

Thank you for your interest in contributing to backplane-cli\! We believe a consistent and intuitive user experience is paramount for our CLI application. This document outlines our guidelines for user experience.

> [!NOTE]
> Not all existing commands currently follow this guide. However, future development and ongoing refactoring efforts will gradually bring them into alignment.

### Goal of backplane-cli

Assist users to get access to managed OSD/ROSA clusters by interacting with backplane-api and configuring local environments.

### Goal of this document

This document provides a guideline on what we should follow when creating a new sub-command and refactoring an existing command.

### Command Structure

A subcommand should look like this:  
APPNAME \[GROUP\] VERB NOUN \--ADJECTIVE  (adapt from [cobra.dev](https://cobra.dev/))

```shell
ocm-backplane cluster login <cluster-id> --manager
## this command doesn't exist, it should be `ocm-backplane login` at this time.
```

We divide the commands into the below groups based on different aspects.

| Group | Description |
| :---- | :---- |
| cluster | related to accessing OSD/ROSA kube-apiserver |
| cloud | related to accessing customer cloud environment |
| scripts | related to managed-scripts |
| accessrequest | related to access request which gets customer approval for certain access |
|  |  |

To add a new command, create it as a sub-command of one of the above group commands, or add another dedicated group if it is a completely new thing.

#### Name and Alias
When naming a command, choose concise words which most related to the function. If multiple words are needed, separate the words by `-`. Eg, `test-job`.

Depends on the situation, provide alias for the command name:
- For common abbreviations, provide alias to improve productive. Eg, `list` \-\> `ls`, `namespace` \-\> `ns`.
- For common synonyms, provide alias to make better user-experience. Eg, `search` \-\> `lookup`.


### Command Help messages

Provide the Use of Short, Long, Example in the subcommand.

**Short**: one sentence of what this command does. Eg,
```
login to a cluster.
```

**Long**: a detailed description of what this command does, what the user should expect from this command.
```
The login command configures the kubeconfig file, enabling access to the specified cluster. It retrieves the necessary URL from the backplane-api and constructs the kubeconfig file. Authentication to the backplane-api is performed using your OCM token.
```

**Example**: provide a few examples of how to use this command, including the common usage of the flags.
```
 backplane login \<id\>
 backplane login %test%
 backplane login \<external\_id\>
 backplane login \--pd \<incident-id\>
```

**README**:

Also, please provide an example of the common usage of the command in the repo's [README usage section](https://github.com/openshift/backplane-cli/blob/main/README.md#usage).


### Command Tunables

#### Global flags

When adding a new flag, it should be a global flag only if all sub-commands of this command, including existing ones, must respect it. Otherwise, add it as a local flag.

Any new sub-command must also respect all existing global flags of its parent command.

Currently, the root global flag we have is `-v, --verbosity` , all subcommands should respect this flag when handing outputs.

#### Local flags

When adding a new subcommand with local flags, follow the same convention as other existing subcommands.

For example, if the new command allows the users to specify the backplane-api url, use `--url` the same as other existing commands.

#### Environment variables

By default, the dependency components respect their environment variables.

- HTTP
- OCM
- kube client

For backplane related environments, if we want to introduce one for backplane, name it with prefix BP\_\*.

The environment variable name should be defined in [info.go](pkg/info/info.go).

#### Config file

Good for storing static facts. eg, some URLs or keys that are not suitable for putting in the public repo.

#### Precedence

If users have multiple ways to set the tunnable, follow this precedence:

Flag \> Environment variables \> Config file

#### Decision guide

To decide whether to use a flag, env or config file, here is a guide.

| Setting Type | CLI Flag | Env Var | Config File | Recommended Use |
| ----- | ----- | ----- | ----- | ----- |
| Frequently changed | Yes | Optional | Optional | Expect different values for different executions. |
| Secret or credential | No | Yes | Optional | Use Env for credential by default. Optionally use the config file. |
| Static or persistent | No | Optional | Yes | Use config files |
| Tweakable default | Yes | Optional | Yes | Use CLI for override, config for base; optionally env |

### Command Input

#### Interactive & non-interactive

Keep in mind that users may run the backplane commands in a script. If you are asking for interactive input, always provide an option to accept the input non-interactively, eg, by flags or environment variables.

Example:  
The elevate command by default accept the reason non-interactively:

```shell
ocm backplane elevate <reason> -- get po -A
```

It only prompts interactive input when specified with \`-n\`.

```shell
ocm backplane elevate -n -- get po -A
```

### Command Output

#### Format

If the command is to retrieve an info for the user, print the output to stdout. Be friendly to command line tools like grep and awk.  
Optionally, provide an option to output json format for programmatically processing.

Example:  
`managedjob` provides an option to output raw response from backplane-api, which is a json format.

```shell
ocm-backplane managedjob create <script name> --raw
```

Command `cloud credentials` provide options to output in different formats.

```go
ocm-backplane cloud credentials
-o, --output string   Format the output of the credentials response. One of text|json|yaml|env (default "text")
```

#### Verbose

Provide verbose output.  The backplane users are mostly with technical backgrounds.
Share more information for users to debug.

**Debug**
Detailed progress of internal processes

**Info**
Major steps in a multi-stage process
Configuration details being used

**Warn**
Deprecated features being used
Non-critical configuration issues

**Error**
Invalid input or arguments
Fatal application errors

Example:
```
DEBU  Running Login Command
DEBU  Checking Backplane Version
WARN  Your Backplane CLI is not up to date. Please run the command 'ocm backplane upgrade' to upgrade to the latest version  Current version=0.1.44 Latest version=0.1.47
DEBU  Extracting Backplane configuration
ERRO  failed to create OCM connection: please ensure you are logged into OCM by using the command "ocm login \--url $ENV"
```

### Error Handing

Throw errors, don't hide errors.

Poor: This one hides the error, the user may be frustrated on what went wrong.

```go
if err != nil {
  return fmt.Errorf("can't find shard url")
}
```

Good: This one returns the underlying error, so the user can troubleshoot easier.

```go
if err != nil {
  return fmt.Errorf("can't find shard url: %v", err)
}
```

### Behavior of a command

#### Validate arguments

Validate the arguments/inputs at the earliest possible.


## Pull Request Guideline

Guideline for making a Pull Request (PR) in the backplane-cli repo.

### Title
The title should follow this format:
```
[Ticket] type: short description
```
Example: `[PROJ-123] feat: add SSM support`

#### Ticket number
The ticket number is optional. If no associated ticket, the title can be `feat: add SSM support`.

#### Type

| Type   | Description                                |
|--------|--------------------------------------------|
| feat   | A new feature or capability                |
| fix    | A bug fix or correction                    |
| chore  | Maintenance tasks, config, or dependencies |
| docs   | Documentation-only changes                 |
| test   | Adding or updating tests                   |
| other  | Anything that doesn't fit the above types  |

If the PR is a new feature with tests, use the type `feat`.


#### Short Description

| Rule                                | Example ✅                         | Example ❌                      |
|-------------------------------------|-----------------------------------|---------------------------------|
| Use imperative mood (command form)  | `add login endpoint`              | `added login endpoint`         |
| Be concise and specific             | `handle missing token error`      | `make login better`            |
| Do not end with a period            | `refactor cloud service`          | `refactor cloud service.`      |
| Use lowercase (except proper nouns) | `add support for OCM login`       | `Add Support For OCM Login`    |

### Squash
The PR template has the below line to squash commits by default, please keep it:
```
/label tide/merge-method-squash
```
