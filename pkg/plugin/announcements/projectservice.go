package announcements

import (
	"github.com/andygrunwald/go-jira"
	bpJira "github.com/openshift/backplane-cli/pkg/jira"
)

type HandoverProjectService struct {
	jiraClient *jira.Client
}

type issueSearcher interface {
	SearchIssues(jql string) ([]jira.Issue, error)
}

func NewHandoverProjectService() (*HandoverProjectService, error) {
	client, err := bpJira.CreateJiraClient()
	if err != nil {
		return nil, err
	}

	return &HandoverProjectService{jiraClient: client}, nil
}

func (ps *HandoverProjectService) SearchIssues(jql string) ([]jira.Issue, error) {
	issues, _, err := ps.jiraClient.Issue.Search(jql, &jira.SearchOptions{
		MaxResults: 50,
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}
