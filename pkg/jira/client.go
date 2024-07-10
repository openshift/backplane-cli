package jira

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
)

// JiraClient is an interface to handle jira functions
type JiraClient interface {
	Connect(baseURL string, jiraToken string) error
	SearchIssue(jql string, options *jira.SearchOptions) (issues []jira.Issue, err error)
	CreateIssue(issue *jira.Issue) (*jira.Issue, error)
}

type DefaultJiraClientImpl struct {
	client *jira.Client
}

// NewClient creates an instance of Jira client and used to connect to the actual jira client.
func NewClient() *DefaultJiraClientImpl {
	return &DefaultJiraClientImpl{}
}

// Connect initiate the Jira connection
func (c *DefaultJiraClientImpl) Connect(baseURL string, jiraToken string) error {

	if baseURL == "" {
		return fmt.Errorf("empty Jira base url")
	}

	if jiraToken == "" {
		return fmt.Errorf("empty Jira token")
	}

	transport := jira.PATAuthTransport{
		Token: jiraToken,
	}
	JiraClient, err := jira.NewClient(transport.Client(), baseURL)

	if err != nil {
		return err
	}
	c.client = JiraClient
	return nil
}

// SearchIssue returns the issues based on JQL
func (c *DefaultJiraClientImpl) SearchIssue(jql string, options *jira.SearchOptions) (issues []jira.Issue, err error) {
	issues, _, err = c.client.Issue.Search(jql, options)
	return issues, err
}

// Create Jira Issue
func (c *DefaultJiraClientImpl) CreateIssue(issue *jira.Issue) (*jira.Issue, error) {
	issue, _, err := c.client.Issue.Create(issue)
	return issue, err
}
