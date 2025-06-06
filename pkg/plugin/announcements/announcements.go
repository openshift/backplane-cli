package announcements

import (
	"fmt"

	"github.com/openshift/backplane-cli/pkg/cli/config"
)

type fieldQuery struct {
	Field    string
	Operator string
	Value    string
}

// relatedHandoverAnnouncements returns related handover announcements queried from JIRA
func relatedHandoverAnnouncements(cluster *clusterRecord) error {
	service, err := NewHandoverProjectService()
	if err != nil {
		return fmt.Errorf("failed to create project service: %v", err)
	}
	return relatedHandoverAnnouncementsWithService(cluster, service)
}

// helper to allow injection of mock in tests
func relatedHandoverAnnouncementsWithService(cluster *clusterRecord, service issueSearcher) error {
	projectKey := JiraHandoverAnnouncementProjectKey

	baseQueries := []fieldQuery{
		{Field: "Cluster ID", Value: cluster.ClusterID, Operator: "~"},
		{Field: "Cluster ID", Value: cluster.ExternalID, Operator: "~"},
	}
	jql := buildJQL(projectKey, baseQueries)
	issues, err := service.SearchIssues(jql)
	if err != nil {
		return fmt.Errorf("failed to get issues: %v", err)
	}

	extendedQueries := []fieldQuery{
		{Field: "Cluster ID", Value: "None,N/A,All", Operator: "~*"},
		{Field: "Customer Name", Value: cluster.OrgName, Operator: "~"},
		{Field: "Products", Value: cluster.Product, Operator: "="},
		{Field: "affectedVersion", Value: formatVersion(cluster.Version), Operator: "~"},
	}

	jql = buildJQL(projectKey, extendedQueries)
	otherIssues, err := service.SearchIssues(jql)
	if err != nil {
		return fmt.Errorf("failed to get issues: %v", err)
	}

	fmt.Printf("Related Handover Announcements:\n")
	for _, i := range otherIssues {
		if isValidMatch(i, *cluster) {
			issues = append(issues, i)
		}
	}

	for _, i := range issues {
		fmt.Printf("[%s]: %+v\n", i.Key, i.Fields.Summary)
		fmt.Printf("- Link: %s/browse/%s\n\n", config.JiraBaseURLDefaultValue, i.Key)
	}

	if len(issues) == 0 {
		fmt.Println("None")
	}
	return nil
}
