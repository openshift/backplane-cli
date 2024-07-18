package jira

import (
	"errors"
	"fmt"

	"github.com/openshift/backplane-cli/pkg/cli/config"

	"github.com/andygrunwald/go-jira"
)

type IssueServiceInterface interface {
	Create(issue *jira.Issue) (*jira.Issue, *jira.Response, error)
	Get(issueID string, options *jira.GetQueryOptions) (*jira.Issue, *jira.Response, error)
	Update(issue *jira.Issue) (*jira.Issue, *jira.Response, error)
	GetTransitions(id string) ([]jira.Transition, *jira.Response, error)
	DoTransition(ticketID, transitionID string) (*jira.Response, error)
}

type IssueServiceGetter interface {
	GetIssueService() (*jira.IssueService, error)
}

type IssueServiceDecorator struct {
	Getter IssueServiceGetter
}

func (decorator *IssueServiceDecorator) Create(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	issueService, err := decorator.Getter.GetIssueService()

	if err != nil {
		return nil, nil, err
	}

	return issueService.Create(issue)
}

func (decorator *IssueServiceDecorator) Get(issueID string, options *jira.GetQueryOptions) (*jira.Issue, *jira.Response, error) {
	issueService, err := decorator.Getter.GetIssueService()

	if err != nil {
		return nil, nil, err
	}

	return issueService.Get(issueID, options)
}

func (decorator *IssueServiceDecorator) Update(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	issueService, err := decorator.Getter.GetIssueService()

	if err != nil {
		return nil, nil, err
	}

	return issueService.Update(issue)
}

func (decorator *IssueServiceDecorator) GetTransitions(id string) ([]jira.Transition, *jira.Response, error) {
	issueService, err := decorator.Getter.GetIssueService()

	if err != nil {
		return []jira.Transition{}, nil, err
	}

	return issueService.GetTransitions(id)
}

func (decorator *IssueServiceDecorator) DoTransition(ticketID, transitionID string) (*jira.Response, error) {
	issueService, err := decorator.Getter.GetIssueService()

	if err != nil {
		return nil, err
	}

	return issueService.DoTransition(ticketID, transitionID)
}

type DefaultIssueServiceGetterImpl struct {
	issueService *jira.IssueService
}

func createIssueService() (*jira.IssueService, error) {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return nil, fmt.Errorf("failed to load backplane config: %v", err)
	}

	if bpConfig.JiraToken == "" {
		return nil, fmt.Errorf("JIRA token is not defined, consider defining it running 'ocm-backplane config set %s <token value>'", config.JiraTokenViperKey)
	}

	transport := jira.PATAuthTransport{
		Token: bpConfig.JiraToken,
	}

	jiraClient, err := jira.NewClient(transport.Client(), bpConfig.JiraBaseURL)

	if err != nil || jiraClient == nil {
		return nil, fmt.Errorf("failed to create the JIRA client: %v", err)
	}

	issueService := jiraClient.Issue

	if issueService == nil {
		return nil, errors.New("no issue service in the JIRA client")
	}

	return issueService, nil
}

func (getter *DefaultIssueServiceGetterImpl) GetIssueService() (*jira.IssueService, error) {
	if getter.issueService == nil {
		issueService, err := createIssueService()

		if err != nil {
			return nil, err
		}

		getter.issueService = issueService
	}

	return getter.issueService, nil
}

var DefaultIssueService IssueServiceInterface = &IssueServiceDecorator{Getter: &DefaultIssueServiceGetterImpl{}}
