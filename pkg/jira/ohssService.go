package jira

import (
	"fmt"
	"strings"

	"github.com/andygrunwald/go-jira"
)

const (
	JiraOHSSProjectKey   = "OHSS"
	CustomFieldClusterID = "customfield_12316349"
)

type OHSSIssue struct {
	ID         string
	Key        string
	Title      string
	ProjectKey string
	WebURL     string
	ClusterID  string
}

type OHSSService struct {
	issueService IssueServiceInterface
}

func NewOHSSService(client IssueServiceInterface) *OHSSService {
	return &OHSSService{
		issueService: client,
	}
}

// GetIssue returns matching issue from OHSS project
func (j *OHSSService) GetIssue(issueID string) (ohssIssue OHSSIssue, err error) {

	if issueID == "" {
		return ohssIssue, fmt.Errorf("empty issue Id")
	}
	issue, _, err := j.issueService.Get(issueID, nil)
	if err != nil {
		return ohssIssue, err
	}
	if issue == nil {
		return ohssIssue, fmt.Errorf("no matching issue for issueID:%s", issueID)
	}

	if issue.Fields != nil {
		if issue.Fields.Project.Key != JiraOHSSProjectKey {
			return ohssIssue, fmt.Errorf("issue %s is not belongs to OHSS project", issueID)
		}
	}
	formatIssue, err := j.formatIssue(*issue)
	if err != nil {
		return ohssIssue, err
	}
	return formatIssue, nil

}

// formatIssue format the JIRA issue to OHSS Issue
func (j *OHSSService) formatIssue(issue jira.Issue) (formatIssue OHSSIssue, err error) {

	formatIssue.ID = issue.ID
	formatIssue.Key = issue.Key
	if issue.Fields != nil {
		clusterID, clusterIDExist := issue.Fields.Unknowns[CustomFieldClusterID]
		if clusterIDExist {
			formatIssue.ClusterID = fmt.Sprintf("%s", clusterID)
		}
		formatIssue.ProjectKey = issue.Fields.Project.Key
		formatIssue.Title = issue.Fields.Summary
	}
	if issue.Self != "" {
		selfSlice := strings.SplitAfter(issue.Self, ".com")
		formatIssue.WebURL = fmt.Sprintf("%s/browse/%s", selfSlice[0], issue.Key)
	}

	return formatIssue, nil
}
