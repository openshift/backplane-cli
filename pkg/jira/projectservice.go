package jira

import (
	"github.com/andygrunwald/go-jira"
)

type ProjectService struct {
	jiraClient *jira.Client
}

func NewProjectService() (*ProjectService, error) {
	client, err := createJiraClient()
	if err != nil {
		return nil, err
	}

	return &ProjectService{jiraClient: client}, nil
}

func (ps *ProjectService) SearchIssues(jql string) ([]jira.Issue, error) {
	issues, _, err := ps.jiraClient.Issue.Search(jql, &jira.SearchOptions{
		MaxResults: 50,
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}
