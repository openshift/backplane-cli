package jira

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
	"github.com/openshift/backplane-cli/pkg/cli/config"
)

const (
	JiraOHSSProjectName            = "OpenShift Hosted SRE Support"
	JqlGetIssueTemplate            = `project = "%s" AND issue = "%s"`
	JqlGetIssuesForClusterTemplate = `(project = "%s" AND "Cluster ID" ~ "%s") 
		OR (project = "%s" AND "Cluster ID" ~ "%s") 
		ORDER BY created DESC`
)

type Jira struct {
	client JiraClient
}

func NewJira(client JiraClient) *Jira {
	return &Jira{
		client: client,
	}
}

func NewJiraFromConfig(bpConfig config.BackplaneConfiguration) (*Jira, error) {
	jiraClient := NewClient()
	err := jiraClient.Connect(bpConfig.JiraBaseUrl, bpConfig.JiraAPIToken)

	if err != nil {
		return nil, err
	}

	return &Jira{
		client: jiraClient,
	}, nil

}

// GetIssue returns matching issue from OHSS project
func (j *Jira) GetIssue(issueID string) (issue jira.Issue, err error) {

	if issueID == "" {
		return issue, fmt.Errorf("empty issue Id")
	}
	jql := fmt.Sprintf(JqlGetIssueTemplate, JiraOHSSProjectName, issueID)
	issues, err := j.client.SearchIssue(jql, nil)
	if err != nil {
		return issue, err
	}
	switch len(issues) {
	case 0:
		return issue, fmt.Errorf("no matching issue for issueID:%s", issueID)
	case 1:
		return issues[0], nil
	default:
		return issue, fmt.Errorf("more than one matching issues for issueID:%s", issueID)
	}
}

// GetJiraIssuesForCluster returns all matching issues for cluster ID
func (j *Jira) GetJiraIssuesForCluster(clusterID string, externalClusterID string) ([]jira.Issue, error) {

	if clusterID == "" && externalClusterID == "" {
		return nil, fmt.Errorf("empty cluster id and external cluster id ")
	}

	jql := fmt.Sprintf(
		JqlGetIssuesForClusterTemplate,
		JiraOHSSProjectName,
		clusterID,
		JiraOHSSProjectName,
		externalClusterID,
	)
	issues, err := j.client.SearchIssue(jql, nil)
	if err != nil {
		return nil, err
	}
	return issues, nil
}
