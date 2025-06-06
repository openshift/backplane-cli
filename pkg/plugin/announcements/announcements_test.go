package announcements

import (
	"errors"
	"strings"
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockIssueSearcher struct {
	mock.Mock
}

func (m *mockIssueSearcher) SearchIssues(jql string) ([]jira.Issue, error) {
	args := m.Called(jql)
	return args.Get(0).([]jira.Issue), args.Error(1)
}

func TestRelatedHandoverAnnouncements_Success(t *testing.T) {
	mockService := new(mockIssueSearcher)

	cluster := &clusterRecord{
		ClusterID:  "123",
		ExternalID: "abc",
		OrgName:    "Red Hat",
		Product:    "OpenShift Dedicated",
		Version:    "4.14.7",
	}

	baseIssues := []jira.Issue{
		{Key: "JIRA-123", Fields: &jira.IssueFields{Summary: "Base issue"}},
	}
	extendedIssues := []jira.Issue{
		{Key: "JIRA-456", Fields: &jira.IssueFields{
			Summary: "Extended issue",
			Unknowns: map[string]interface{}{
				productCustomField: []interface{}{
					map[string]interface{}{"value": "OpenShift Dedicated"},
				},
				customerNameCustomField: "Red Hat",
			},
			AffectsVersions: []*jira.AffectsVersion{{Name: "4.14.0"}},
		}},
	}

	mockService.On("SearchIssues", mock.MatchedBy(func(jql string) bool {
		return strings.Contains(jql, "Cluster ID")
	})).Return(baseIssues, nil).Once()

	mockService.On("SearchIssues", mock.MatchedBy(func(jql string) bool {
		return strings.Contains(jql, "Customer Name")
	})).Return(extendedIssues, nil).Once()

	err := relatedHandoverAnnouncementsWithService(cluster, mockService)
	assert.NoError(t, err)
	mockService.AssertExpectations(t)
}

func TestRelatedHandoverAnnouncements_SearchFailure(t *testing.T) {
	mockService := new(mockIssueSearcher)
	mockService.On("SearchIssues", mock.Anything).Return([]jira.Issue{}, errors.New("search failed")).Once()

	cluster := &clusterRecord{}

	err := relatedHandoverAnnouncementsWithService(cluster, mockService)
	assert.ErrorContains(t, err, "failed to get issues")
	mockService.AssertExpectations(t)
}
