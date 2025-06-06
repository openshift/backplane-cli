package login

import (
	"fmt"
	"strings"

	jiraclient "github.com/andygrunwald/go-jira"
	"github.com/openshift/backplane-cli/pkg/jira"
	"github.com/openshift/backplane-cli/pkg/ocm"
)

type fieldQuery struct {
	Field    string
	Operator string
	Value    string
}

const (
	JiraHandoverAnnouncementProjectKey = "SRE Platform HandOver Announcements"
	JiraBaseURL                        = "https://issues.redhat.com"
	productCustomField                 = "customfield_12319040"
	customerNameCustomField            = "customfield_12310160"
)

func hasProduct(items []interface{}, target string) bool {
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if val, ok := m["value"].(string); ok && val == target {
				return true
			}
		}
	}
	return false
}
func buildJQL(projectKey string, filters []fieldQuery) string {
	var conditions []string
	for _, q := range filters {
		if q.Operator == "~*" {
			values := strings.Split(q.Value, ",")
			var orParts []string
			for _, v := range values {
				orParts = append(orParts,
					fmt.Sprintf(`(project = "%s" AND "%s" ~ "%s")`, projectKey, q.Field, strings.TrimSpace(v)))
			}
			conditions = append(conditions, "("+strings.Join(orParts, " OR ")+")")
		} else if q.Operator == "in" {
			conditions = append(conditions,
				fmt.Sprintf(`(project = "%s" AND "%s" in (%s))`, projectKey, q.Field, q.Value),
			)
		} else {
			conditions = append(conditions,
				fmt.Sprintf(`(project = "%s" AND "%s" %s "%s")`, projectKey, q.Field, q.Operator, q.Value),
			)
		}
	}
	return "(" + strings.Join(conditions, " OR ") + ") AND status != Closed ORDER BY created DESC"
}

func formatVersion(version string) string {
	versionParts := strings.Split(version, ".")
	versionPrefix := version
	if len(versionParts) >= 2 {
		versionPrefix = fmt.Sprintf("%s.%s", versionParts[0], versionParts[1])
	}
	return versionPrefix
}

func RelatedHandoverAnnouncements(cluster *ocm.ClusterRecord) (err error) {
	projectService, err := jira.NewProjectService()
	if err != nil {
		return fmt.Errorf("failed to create project service: %v", err)
	}

	projectKey := JiraHandoverAnnouncementProjectKey

	baseQueries := []fieldQuery{
		{Field: "Cluster ID", Value: cluster.ClusterID, Operator: "~"},
		{Field: "Cluster ID", Value: cluster.ExternalID, Operator: "~"},
	}
	jql := buildJQL(projectKey, baseQueries)
	issues, err := projectService.SearchIssues(jql)

	if err != nil {
		return fmt.Errorf("failed to get issues: %v", err)
	}

	extededQueries := []fieldQuery{
		{Field: "Cluster ID", Value: "None,N/A,All", Operator: "~*"},
		{Field: "Customer Name", Value: cluster.OrgName, Operator: "~"},
		{Field: "Products", Value: cluster.Product, Operator: "="},
		{Field: "affectedVersion", Value: formatVersion(cluster.Version), Operator: "~"},
	}

	jql = buildJQL(projectKey, extededQueries)
	otherIssues, err := projectService.SearchIssues(jql)

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
		fmt.Printf("- Link: %s/browse/%s\n\n", JiraBaseURL, i.Key)
	}

	if len(issues) == 0 {
		fmt.Println("None")
	}
	return nil
}

func isValidMatch(i jiraclient.Issue, cluster ocm.ClusterRecord) bool {
	isIgnored := func(val string) bool {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "none" || val == "n/a" || val == "all" || val == ""
	}

	hasMatchingValue := func(items []interface{}, expected string) bool {
		expected = strings.ToLower(strings.TrimSpace(expected))
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				if val, ok := m["value"].(string); ok {
					val = strings.ToLower(strings.TrimSpace(val))
					if val == expected {
						return true
					}
				}
			}
		}
		return false
	}

	productRaw := i.Fields.Unknowns[productCustomField]
	versionRaw := i.Fields.AffectsVersions
	nameRaw := i.Fields.Unknowns[customerNameCustomField]

	productMatch := false
	if items, ok := productRaw.([]interface{}); ok {
		productMatch = hasMatchingValue(items, cluster.Product)
	}
	if !productMatch {
		return false
	}

	versionMatch := false
	clusterFormattedVersion := formatVersion(cluster.Version)

	if versionRaw != nil {
		for _, v := range versionRaw {
			if v != nil {
				vFormatted := formatVersion(v.Name)
				if vFormatted == clusterFormattedVersion || isIgnored(v.Name) {
					versionMatch = true
					break
				}
			}
		}
	}

	nameMatch := false
	if nameStr, ok := nameRaw.(string); ok {
		parts := strings.Split(nameStr, ";")
		for _, part := range parts {
			val := strings.TrimSpace(part)
			if val == cluster.OrgName || isIgnored(val) {
				nameMatch = true
				break
			}
		}
	}

	return versionMatch || nameMatch
}
