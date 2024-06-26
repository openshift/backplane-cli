package accessrequest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/andygrunwald/go-jira"
	acctrspv1 "github.com/openshift-online/ocm-sdk-go/accesstransparency/v1"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/pkg/backplaneapi"
	backplaneapiMock "github.com/openshift/backplane-cli/pkg/backplaneapi/mocks"
	"github.com/openshift/backplane-cli/pkg/ocm"
	ocmMock "github.com/openshift/backplane-cli/pkg/ocm/mocks"
	"github.com/openshift/backplane-cli/pkg/utils"
	utilsMocks "github.com/openshift/backplane-cli/pkg/utils/mocks"
)

const testDesc = "accessrequest package"

type IssueMatcher struct {
	closure func(issue *jira.Issue)
}

func (matcher IssueMatcher) Matches(x interface{}) bool {
	issue := x.(*jira.Issue)
	matcher.closure(issue)

	return true
}

func (matcher IssueMatcher) String() string {
	return "IssueMatcher"
}

func writeConfig(jsonData []byte) {
	tempDir := os.TempDir()
	bpConfigPath := filepath.Join(tempDir, "mock.json")
	tempFile, err := os.Create(bpConfigPath)
	Expect(err).To(BeNil())

	_, err = tempFile.Write(jsonData)
	Expect(err).To(BeNil())

	os.Setenv("BACKPLANE_CONFIG", bpConfigPath)
}

var _ = Describe(testDesc, func() {
	var (
		mockCtrl         *gomock.Controller
		mockClientUtil   *backplaneapiMock.MockClientUtils
		mockOcmInterface *ocmMock.MockOCMInterface
		mockIssueService *utilsMocks.MockIssueServiceInterface

		clusterID       string
		ocmEnv          *cmv1.Environment
		reason          string
		issueID         string
		issue           *jira.Issue
		duration        time.Duration
		durationStr     string
		accessRequestID string
		accessRequest   *acctrspv1.AccessRequest
	)

	BeforeEach(func() {
		var err error

		mockCtrl = gomock.NewController(GinkgoT())

		mockClientUtil = backplaneapiMock.NewMockClientUtils(mockCtrl)
		backplaneapi.DefaultClientUtils = mockClientUtil

		mockOcmInterface = ocmMock.NewMockOCMInterface(mockCtrl)
		ocm.DefaultOCMInterface = mockOcmInterface

		mockIssueService = utilsMocks.NewMockIssueServiceInterface(mockCtrl)
		utils.DefaultIssueService = mockIssueService

		clusterID = "cluster-12345678"

		ocmEnv, _ = cmv1.NewEnvironment().BackplaneURL("https://dummy.api").Build()

		reason = "Some reason"

		issueID = "SDAINT-12345"

		issue = &jira.Issue{
			Fields: &jira.IssueFields{
				Description: "Access request tracker",
				Type: jira.IssueType{
					Name: "Story",
				},
				Project: jira.Project{
					Key: "SDAINT",
				},
				Summary: fmt.Sprintf("Access request tracker for cluster '%s'", clusterID),
			},
			Key: issueID,
		}

		duration = 1 * time.Hour

		durationStr = "1h0m0s"

		accessRequestID = "req-123"
		accessRequestBuilder := acctrspv1.NewAccessRequest().ID(accessRequestID).HREF(accessRequestID).Justification(reason).
			InternalSupportCaseId(issueID).Duration(durationStr).Status(acctrspv1.NewAccessRequestStatus().State(acctrspv1.AccessRequestStatePending))
		accessRequest, err = accessRequestBuilder.Build()

		Expect(err).To(BeNil())
	})

	Context("get access request", func() {
		It("should fail when access protection is disabled", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(nil, clusterID).Return(false, nil).Times(1)

			returnedAccessRequest, err := GetAccessRequest(nil, clusterID)

			Expect(returnedAccessRequest).To(BeNil())
			Expect(err.Error()).Should(ContainSubstring("access protection is not enabled"))
		})

		It("should fail when access protection cannot be retrieved from OCM", func() {
			mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(nil, clusterID).Return(true, errors.New("some error")).Times(1)

			returnedAccessRequest, err := GetAccessRequest(nil, clusterID)

			Expect(returnedAccessRequest).To(BeNil())
			Expect(err.Error()).Should(ContainSubstring("unable to determine if access protection is enabled or not"))
		})

		Context("access protection is enabled", func() {
			BeforeEach(func() {
				mockOcmInterface.EXPECT().IsClusterAccessProtectionEnabled(nil, clusterID).Return(true, nil).Times(1)
			})

			It("should fail when access request cannot be retrieved from OCM", func() {
				mockOcmInterface.EXPECT().GetClusterActiveAccessRequest(nil, clusterID).Return(nil, errors.New("some error")).Times(1)

				returnedAccessRequest, err := GetAccessRequest(nil, clusterID)

				Expect(returnedAccessRequest).To(BeNil())
				Expect(err.Error()).To(Equal("some error"))
			})

			It("should return nil if there is no access request in OCM", func() {
				mockOcmInterface.EXPECT().GetClusterActiveAccessRequest(nil, clusterID).Return(nil, nil).Times(1)

				returnedAccessRequest, err := GetAccessRequest(nil, clusterID)

				Expect(returnedAccessRequest).To(BeNil())
				Expect(err).To(BeNil())
			})

			It("should succeed and return the access request if there is one in OCM", func() {
				mockOcmInterface.EXPECT().GetClusterActiveAccessRequest(nil, clusterID).Return(accessRequest, nil).Times(1)

				returnedAccessRequest, err := GetAccessRequest(nil, clusterID)

				Expect(returnedAccessRequest).To(Equal(accessRequest))
				Expect(err).To(BeNil())
			})
		})
	})

	Context("create access request", func() {
		It("should fail when env cannot be retrieved from OCM", func() {
			mockOcmInterface.EXPECT().GetOCMEnvironment().Return(nil, errors.New("some error")).Times(1)

			returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, issueID, duration)

			Expect(returnedAccessRequest).To(BeNil())
			Expect(err.Error()).To(Equal("some error"))
		})

		Context("env successfully retrieved from OCM", func() {
			BeforeEach(func() {
				mockOcmInterface.EXPECT().GetOCMEnvironment().Return(ocmEnv, nil).AnyTimes()
			})

			It("should fail when the backplane config is invalid", func() {
				writeConfig([]byte("Hello World"))

				returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "TST-12345", duration)

				Expect(returnedAccessRequest).To(BeNil())
				Expect(err.Error()).Should(ContainSubstring("invalid character 'H' looking for beginning of value"))
			})

			Context("backplane config is valid", func() {
				BeforeEach(func() {
					writeConfig([]byte("{}")) // Defaults will be used
				})

				Context("notification issue ID passed by the caller", func() {
					It("should fail when the issue project is not in the config", func() {
						returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "AAA-12345", duration)

						Expect(returnedAccessRequest).To(BeNil())
						Expect(err.Error()).Should(ContainSubstring("issue does not belong to one of the 'SDAINT' test JIRA projects"))
					})

					Context("issue project is known from the config", func() {
						It("should succeed even if the JIRA token is not defined in the config as the issue existence cannot be disproved", func() {
							mockOcmInterface.EXPECT().CreateClusterAccessRequest(nil, clusterID, reason, issueID, durationStr).Return(accessRequest, nil).Times(1)

							returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, issueID, duration)

							Expect(returnedAccessRequest).To(Equal(accessRequest))
							Expect(err).To(BeNil())
						})

						Context("JIRA token is defined in the config", func() {
							BeforeEach(func() {
								writeConfig([]byte(`{"jira-token": "xxxxxx"}`))
							})

							It("should fail when the issue cannot be retrieved from JIRA", func() {
								mockIssueService.EXPECT().Get(issueID, nil).Return(nil, nil, errors.New("some error")).Times(1)

								returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, issueID, duration)

								Expect(returnedAccessRequest).To(BeNil())
								Expect(err.Error()).Should(ContainSubstring("some error"))
							})

							Context("issue is defined in JIRA", func() {
								BeforeEach(func() {
									mockIssueService.EXPECT().Get(issueID, nil).Return(issue, nil, nil).AnyTimes()
								})

								Context("access request can be created in OCM", func() {
									BeforeEach(func() {
										mockOcmInterface.EXPECT().CreateClusterAccessRequest(nil, clusterID, reason, issueID, durationStr).Return(accessRequest, nil).Times(1)
									})

									It("should succeed and create the access request in OCM even if the issue cannot later be transitioned or updated in JIRA", func() {
										mockIssueService.EXPECT().GetTransitions(issueID).Return([]jira.Transition{}, nil, errors.New("some error")).Times(1)
										mockIssueService.EXPECT().Update(gomock.Any()).Return(nil, nil, errors.New("some other error")).Times(1)

										returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, issueID, duration)

										Expect(returnedAccessRequest).To(Equal(accessRequest))
										Expect(err).To(BeNil())
									})
								})
							})
						})
					})
				})

				Context("notification issue to be created", func() {
					It("should fail when the issue cannot be created in JIRA", func() {
						mockIssueService.EXPECT().Create(gomock.Any()).Return(nil, nil, errors.New("some error")).Times(1)

						returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "", duration)

						Expect(returnedAccessRequest).To(BeNil())
						Expect(err.Error()).Should(ContainSubstring("some error"))
					})

					Context("issue can be created in JIRA", func() {
						BeforeEach(func() {
							mockIssueService.EXPECT().Create(IssueMatcher{func(receivedIssue *jira.Issue) {
								Expect(receivedIssue.Key).To(Equal(""))
								Expect(receivedIssue.Fields).To(Equal(issue.Fields))
							}}).Return(issue, nil, nil).AnyTimes()
						})

						Context("access request cannot be created in OCM", func() {
							BeforeEach(func() {
								mockOcmInterface.EXPECT().CreateClusterAccessRequest(nil, clusterID, reason, issueID, durationStr).Return(nil, errors.New("some error")).Times(1)
							})

							It("should fail and not close the issue if not possible in JIRA", func() {
								mockIssueService.EXPECT().GetTransitions(issueID).Return([]jira.Transition{}, nil, errors.New("some other error")).Times(1)

								returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "", duration)

								Expect(returnedAccessRequest).To(BeNil())
								Expect(err.Error()).Should(ContainSubstring("some error"))
								Expect(err.Error()).ShouldNot(ContainSubstring("some other error"))
							})

							It("should fail and close the issue if possible in JIRA", func() {
								mockIssueService.EXPECT().GetTransitions(issueID).Return([]jira.Transition{{ID: "42", Name: "Closed"}}, nil, nil).Times(1)
								mockIssueService.EXPECT().DoTransition(issueID, "42").Return(nil, nil).Times(1)

								returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "", duration)

								Expect(returnedAccessRequest).To(BeNil())
								Expect(err.Error()).Should(ContainSubstring("some error"))
								Expect(err.Error()).ShouldNot(ContainSubstring("some other error"))
							})
						})

						Context("access request can be created in OCM", func() {
							BeforeEach(func() {
								mockOcmInterface.EXPECT().CreateClusterAccessRequest(nil, clusterID, reason, issueID, durationStr).Return(accessRequest, nil).Times(1)
							})

							It("should succeed and create the access request in OCM and not update the issue if not possible in JIRA", func() {
								mockIssueService.EXPECT().GetTransitions(issueID).Return([]jira.Transition{}, nil, errors.New("some error")).Times(1)
								mockIssueService.EXPECT().Update(gomock.Any()).Return(nil, nil, errors.New("some other error")).Times(1)

								returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "", duration)

								Expect(returnedAccessRequest).To(Equal(accessRequest))
								Expect(err).To(BeNil())
							})

							It("should succeed and create the access request in OCM and update the issue if possible in JIRA", func() {
								mockIssueService.EXPECT().GetTransitions(issueID).Return([]jira.Transition{{ID: "68", Name: "In Progress"}}, nil, nil).Times(1)
								mockIssueService.EXPECT().DoTransition(issueID, "68").Return(nil, nil).Times(1)
								mockIssueService.EXPECT().Update(IssueMatcher{func(receivedIssue *jira.Issue) {
									Expect(receivedIssue.Key).To(Equal(issueID))
									Expect(receivedIssue.Fields).ToNot(BeNil())
									Expect(receivedIssue.Fields.Description).Should(ContainSubstring(accessRequestID))
									Expect(receivedIssue.Fields.Description).Should(ContainSubstring("In Progress"))
								}}).Return(issue, nil, nil).Times(1)

								returnedAccessRequest, err := CreateAccessRequest(nil, clusterID, reason, "", duration)

								Expect(returnedAccessRequest).To(Equal(accessRequest))
								Expect(err).To(BeNil())
							})
						})
					})
				})
			})
		})
	})
})

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, testDesc)
}
