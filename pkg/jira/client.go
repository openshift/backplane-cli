package jira

// JiraClient is an interface to handle
type JiraClient interface {
	GetConnection()
	GetIssue()
	CreateIssue()
}
