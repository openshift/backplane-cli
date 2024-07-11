package jira

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
	"github.com/openshift/backplane-cli/pkg/cli/config"
)

const (
	JiraOHSSProjectID              = "OHSS"
	CustomFieldClusterID           = "customfield_12316349"
	JqlGetIssueTemplate            = `project = "%s" AND issue = "%s"`
	JqlGetIssuesForClusterTemplate = `(project = "%s" AND "Cluster ID" ~ "%s") 
		OR (project = "%s" AND "Cluster ID" ~ "%s") 
		ORDER BY created DESC`
)

type OHSSIssue struct {
	ID        string
	Title     string
	ProjectID string
	WebURL    string
	ClusterID string
}

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
	err := jiraClient.Connect(bpConfig.JiraBaseURL, bpConfig.JiraAPIToken)

	if err != nil {
		return nil, err
	}

	return &Jira{
		client: jiraClient,
	}, nil

}

// GetIssue returns matching issue from OHSS project
func (j *Jira) GetIssue(issueID string) (ohssIssue OHSSIssue, err error) {

	if issueID == "" {
		return ohssIssue, fmt.Errorf("empty issue Id")
	}
	issue, err := j.client.GetIssue(issueID, nil)
	if err != nil {
		return ohssIssue, err
	}
	if issue != nil {
		formatIssue, err := j.formatIssue(*issue)
		if err != nil {
			return ohssIssue, err
		}
		return formatIssue, nil
	} else {
		return ohssIssue, fmt.Errorf("no matching issue for issueID:%s", issueID)

	}

}

// GetJiraIssuesForCluster returns all matching issues for cluster ID
func (j *Jira) GetJiraIssuesForCluster(clusterID string, externalClusterID string) ([]jira.Issue, error) {

	if clusterID == "" && externalClusterID == "" {
		return nil, fmt.Errorf("empty cluster id and external cluster id ")
	}

	jql := fmt.Sprintf(
		JqlGetIssuesForClusterTemplate,
		JiraOHSSProjectID,
		clusterID,
		JiraOHSSProjectID,
		externalClusterID,
	)
	issues, err := j.client.SearchIssues(jql, nil)
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// formatIssue for
func (j *Jira) formatIssue(issue jira.Issue) (formatIssue OHSSIssue, err error) {

	formatIssue.ID = issue.ID
	if issue.Fields != nil {
		clusterID, clusterIDExist := issue.Fields.Unknowns[CustomFieldClusterID]
		if clusterIDExist {
			formatIssue.ClusterID = fmt.Sprintf("%s", clusterID)
		}
	}
	formatIssue.WebURL = issue.Self
	formatIssue.Title = issue.Fields.Summary
	formatIssue.ProjectID = issue.Fields.Project.ID
	return formatIssue, nil
}
