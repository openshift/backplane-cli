package announcements

import (
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/stretchr/testify/assert"
)

func TestDetermineClusterProduct(t *testing.T) {
	assert.Equal(t, "Red Hat OpenShift on AWS with Hosted Control Planes", determineClusterProduct("rosa", true))
	assert.Equal(t, "Red Hat OpenShift on AWS", determineClusterProduct("rosa", false))
	assert.Equal(t, "OpenShift Dedicated", determineClusterProduct("osd", false))
	assert.Equal(t, "", determineClusterProduct("unknown", false))
}

func TestHasProduct(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"value": "OpenShift Dedicated"},
		map[string]interface{}{"value": "Something Else"},
	}
	assert.True(t, hasProduct(items, "OpenShift Dedicated"))
	assert.False(t, hasProduct(items, "Unknown"))
}

func TestBuildJQL(t *testing.T) {
	filters := []fieldQuery{
		{Field: "Cluster ID", Operator: "~", Value: "abc"},
		{Field: "Customer Name", Operator: "~", Value: "Red Hat"},
	}
	jql := buildJQL("TEST", filters)
	assert.Contains(t, jql, `"Cluster ID" ~ "abc"`)
	assert.Contains(t, jql, `"Customer Name" ~ "Red Hat"`)
	assert.Contains(t, jql, "project = \"TEST\"")
}

func TestFormatVersion(t *testing.T) {
	assert.Equal(t, "4.14", formatVersion("4.14.7"))
	assert.Equal(t, "4.15", formatVersion("4.15"))
	assert.Equal(t, "4.16", formatVersion("4.16.2.1"))
	assert.Equal(t, "4.14", formatVersion("4.14"))
}

func TestIsValidMatch(t *testing.T) {
	cluster := clusterRecord{
		OrgName: "Red Hat",
		Product: "OpenShift Dedicated",
		Version: "4.14.7",
	}

	issue := jira.Issue{
		Fields: &jira.IssueFields{
			Unknowns: map[string]interface{}{
				productCustomField: []interface{}{
					map[string]interface{}{"value": "OpenShift Dedicated"},
				},
				customerNameCustomField: "Red Hat",
			},
			AffectsVersions: []*jira.AffectsVersion{{Name: "4.14.0"}},
		},
	}

	assert.True(t, isValidMatch(issue, cluster))

	issue.Fields.Unknowns[customerNameCustomField] = "Other"
	assert.True(t, isValidMatch(issue, cluster)) // version match

	issue.Fields.AffectsVersions = []*jira.AffectsVersion{}
	assert.False(t, isValidMatch(issue, cluster))
}
