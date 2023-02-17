# Backplane-CLI

## What it does
Backplane-cli is a CLI tool to interact with Backplane-api, which is a management service that consolidates all access to OSD and ROSA clusters and cloud provider resources. Backplane handles authentication and authorization through OCM profiles, practices least privilege access to those OSD/ROSA clusters and cloud resources, and provides auditing capabilities for compliance and security.


## How backplane-cli works

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

### Managed Job

A managed job is the running instance of a managed script in an OSD/ROSA cluster. Not to be confused with k8s job resource.

Managed jobs run as pods in the `openshift-backplane-managed-scripts` namespace.

When run `ocm backplane managedjob ...`, it will:

- Retrive the logged-in cluster ID.
- Check if the cluster is not hibernating.
- Create a managed job, `ocm backplane managedjob create <script_canonical_name>`:
  - Script canonical name is vital to retrieve the relevant script from the [managed-scripts](https://github.com/openshift/managed-scripts) repo.
  - Parses the canonical name and parameters(if any) to be sent as JSON payload to the backplane service.
  - Sends a POST request to the backplane service
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
  - Sends a DELETE request to thr backplane service: `DELETE https://<backplane-url>/backplane/{clusterId}/job/{jobId}`

