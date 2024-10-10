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
		mockCtrl         *gomock.Controller
		mockIssueService *jiraMock.MockIssueServiceInterface
		ohssService      *OHSSService
		testOHSSID       string
		testIssue        jira.Issue
		issueFields      *jira.IssueFields
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockIssueService = jiraMock.NewMockIssueServiceInterface(mockCtrl)
		ohssService = NewOHSSService(mockIssueService)
		testOHSSID = "OHSS-1000"
		issueFields = &jira.IssueFields{Project: jira.Project{Key: JiraOHSSProjectKey}}
		testIssue = jira.Issue{ID: testOHSSID, Fields: issueFields}

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("When Jira client executes", func() {
		It("Should return one issue", func() {

			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(&testIssue, nil, nil).Times(1)

			issue, err := ohssService.GetIssue(testOHSSID)
			Expect(err).To(BeNil())
			Expect(issue.ID).To(Equal(testOHSSID))
			Expect(issue.ProjectKey).To(Equal(JiraOHSSProjectKey))
		})

		It("Should return error for issue not belongs to OHSS project", func() {

			nonOHSSfields := &jira.IssueFields{Project: jira.Project{Key: "NON-OHSS"}}
			nonOHSSIssue := jira.Issue{ID: testOHSSID, Fields: nonOHSSfields}
			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(&nonOHSSIssue, nil, nil).Times(1)

			_, err := ohssService.GetIssue(testOHSSID)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("issue OHSS-1000 is not belongs to OHSS project"))
		})

		It("Should return error for empty issue", func() {

			mockIssueService.EXPECT().Get(testOHSSID, nil).Return(nil, nil, nil).Times(1)

			_, err := ohssService.GetIssue(testOHSSID)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("no matching issue for issueID:OHSS-1000"))
		})
	})
})
