package jira

import (
	"github.com/andygrunwald/go-jira"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	jiraMock "github.com/openshift/backplane-cli/pkg/jira/mocks"
)

var _ = Describe("Jira", func() {
	var (
		mockCtrl       *gomock.Controller
		mockJiraClient *jiraMock.MockJiraClient
		jiraClient     *Jira
		testOHSSID     string
		testIssue      jira.Issue
		//testGetJql     string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockJiraClient = jiraMock.NewMockJiraClient(mockCtrl)
		jiraClient = NewJira(mockJiraClient)
		testOHSSID = "OHSS-1000"
		testIssue = jira.Issue{ID: testOHSSID}
		//testGetJql = fmt.Sprintf(JqlGetIssueTemplate, JiraOHSSProjectName, testOHSSID)

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("When Jira client executes", func() {
		It("Should return one issue", func() {

			mockJiraClient.EXPECT().GetIssue(testOHSSID, nil).Return(&testIssue, nil).Times(1)

			issue, err := jiraClient.GetIssue(testOHSSID)
			Expect(err).To(BeNil())
			Expect(issue.ID).To(Equal(testOHSSID))
		})
		It("Should return error for empty issue", func() {

			mockJiraClient.EXPECT().GetIssue(testOHSSID, nil).Return(nil, nil).Times(1)

			_, err := jiraClient.GetIssue(testOHSSID)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("no matching issue for issueID:OHSS-1000"))
		})
	})
})
