# Testing a draft managed script

When developing a new [managed script](https://github.com/openshift/managed-scripts), you often want to run your draft before it is merged.

Previously this was done with `ocm backplane testJob create/get/logs`, which required the backplane API to build and run a test job. Those subcommands are now **deprecated**.

The recommended way is `ocm backplane testJob render`, which generates the Kubernetes YAML (ServiceAccount, RBAC and Pod) for your script **locally** — no backplane API call is made. You then apply it directly on a non-production cluster where you have cluster-admin access.

## Example

Assuming your draft script lives in a directory that contains a `metadata.yaml` and the script itself (the same layout used in the [managed-scripts](https://github.com/openshift/managed-scripts) repo):

```bash
# 1. Log in to a non-production cluster with cluster-admin access
oc login <cluster-api-url>

# 2. Render the YAML from your script directory
cd scripts/SREP/example
ocm backplane testJob render -p VAR1=val1 > test-job.yaml

# 3. Review the generated YAML, then apply it
oc apply -f test-job.yaml

# 4. Watch the job and inspect its logs with standard oc commands
oc -n openshift-backplane-managed-scripts get pods
oc -n openshift-backplane-managed-scripts logs <pod-name>

# 5. Clean up after testing
oc delete -f test-job.yaml
```

## Useful flags

| Flag | Description |
| ---- | ----------- |
| `-p, --params 'VAR1=val1'`        | Pass parameters to the script (repeatable, e.g. `-p 'VAR1=val1' -p VAR2=val2`). |
| `-s, --source-dir <dir>`          | Directory of the script to render (defaults to the current directory).          |
| `-i, --base-image-override <img>` | Container image used to run the script. Defaults to the latest managed-scripts image resolved from GitHub. |
| `-o, --output <file>`             | Write the rendered YAML to a file instead of stdout.                            |
