# Design

## How backplane-cli works

### OCM configurations

Like OCM, `backplane-cli` reads OCM configuration from the path specified by the environment variable `OCM_CONFIG`. If `OCM_CONFIG` is not set, it will read `$HOME/.ocm.json` by default.

### Login

When executing `ocm backplane login <cluster-id>`, it will:

- Lookup cluster-id from OCM if the provided argument is a cluster name or an external cluster id.
- Send a GET request to backplane service: `GET https://<backplane-url>//backplane/login/<cluster-id>/`. The backplane service will validate the cluster-id, and return back a proxy url for the cluster. The proxy url is like `https://<backplane-url>/backplane/cluster/<cluster-id>/`.
- Construct the `kubeconfig` file:
  - Create a script file in `$HOME/.kube/ocm-token` for later use. As backplane uses OCM token for authentication, the `ocm-token` script will call `ocm token` to get a fresh access token, and feed to the `oc` command.
  - Create cluster in `kubeconfig`. The `server` of the cluster points to the proxy url just received.
  - Create user in `kubeconfig`. The user has an [ExecConfig](https://godoc.org/k8s.io/client-go/tools/clientcmd/api#ExecConfig) pointing to the script just created.
  - Create a context using the cluster, namespace passed as an argument( default: default ) & user just created, and set it to current context.

### Logout

After you are finished, you can logout of the cluster.

```
$ ocm backplane logout
```

### Project

When executing `ocm backplane project <project-name>`, it will:

- Manipulate the `kubeconfig` and set the namespace of the current context to `project-name`.

This serves as a workaround for the [oc command issue](https://github.com/openshift/oc/issues/647).

### Console

When executing `ocm backplane console`, it will:

- Load the current kubeconfig.
- Lookup the console image from the cluster, if not specified.
- Fetch the pull secret from OCM and store it to `$HOME/.kube/backplane-pull-secret.json`, if file does not exist.
- Start the console container with a random available port, pointing the API to backplane.

### Cloud Console

Backplane supports logging into the cluster's cloud provider console. This makes it possible to perform operations such as debugging an issue, troubleshooting a customer misconfiguration, or directly access the underlying cloud infrastructure.

When run `ocm backplane cloud console <CLUSTERID|EXTERNAL_ID|CLUSTER_NAME|CLUSTER_NAME_SEARCH>`, it will:

- If the provided argument is a cluster name or an external cluster ID, retrieve the cluster ID and cluster name of the target cluster via OCM.
- Send a GET request to backplane service: `GET https://<backplane-url>/backplane/cloud/console/{clusterId}`
- The backplane service will validate the cluster ID, and return back the cloud provider creds of the cluster.
- Parse the retrieved cluster's cloud console creds to segregate the console link.
- Output the retrieved cloud console link to the backplane user. If the output format is not explicitly specified as an option, it defaults to **text** format.

  #### How to

- Login to the target cluster via backplane as the above.
- Run the below command and it will launch the cloud console of the current logged in cluster.
  ```
  $ ocm backplane cloud console
  Console Link:
  Link: https://xxxxx
  ```
- Follow the above link to access the console.

  #### Open in browser

  You can directly open the console in browser by adding flag -b or setting environment variable `BACKPLANE_DEFAULT_OPEN_BROWSER=true`. Example,

  When running this command, it will open the console in the browser automatically.

  ```
  $ ocm backplane cloud console -b
  ```

  Or set the environment variable

  ```
  $ export BACKPLANE_DEFAULT_OPEN_BROWSER=true
  $ ocm backplane cloud console
  ```

### Managed Scripts

- Backplane scripts allows backplane users to run pre-defined scripts that require privileged permissions on OSD clusters.
- It allows SRE/CEE to execute pre-approved, higher privileged scripts without admin access, hence no access elevation alert will be triggered.
- Each script has limited RBAC permissions, this is done via a metadata file associated with the script.
- These scripts are stored in a separate repository called [managed-scripts](https://github.com/openshift/managed-scripts)

When run `ocm backplane scripts list` or `ocm backplane scripts describe <script_name>`, it will:

- Send a GET request to the backplane service: `GET https://<backplane-url>/backplane/script`
- Parse the response retrieved from backplane service and display the script(s).

### Managed Job

A managed job is the running instance of a managed script in an OSD/ROSA cluster. Not to be confused with k8s job resource.

Managed jobs run as pods in the `openshift-backplane-managed-scripts` namespace.

When run `ocm backplane managedjob ...`, it will:

- Retrive the logged-in cluster ID.
- Check if the cluster is not hibernating.
- Create a managed job, `ocm backplane managedjob create <script_canonical_name>`:
  - Script canonical name is vital to retrieve the relevant script from the [managed-scripts](https://github.com/openshift/managed-scripts) repo.
  - Parses the canonical name and parameters(if any) to be sent as JSON payload to the backplane service.
  - Sends a POST request to the backplane service: `POST https://<backplane-url>/backplane/script/{clusterId}/job`
  - The backplane service will validate the clusterID and the JSON payload and trigger the creation of a job pod and return a response body comprising of a message along with the job ID of the newly created managed job.
- Fetch managed jobs, `ocm backplane managedjob get`:
  - Retrieves all the managed jobs running in the namespace the backplane user has access to.
  - If the job ID is provided as an argument, sends a GET request to the backplane service: `GET https://<backplane-url>/backplane/script/{clusterId}/job/{jobId}`
  - If not then fetch all jobs, sends a GET request to the backplane service: `GET https://<backplane-url>/backplane/script/{clusterId}/job`
  - Parses the response and displays managed job(s) running in the cluster.
- Fetch managed job logs, `ocm backplane managedjob logs <jobID>`:
  - Sends a GET request to the backplane service: `GET https://<backplane-url>/backplane/{clusterId}/job/{jobId}/logs`
  - Parses the response and displays the managed job logs.
- Delete a managed job, `ocm backplane delete <jobName>`:
  - Stops a running job and deletes all of its resources (including logs).


