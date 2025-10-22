package accessrequest

import (
	"fmt"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira"
	ocmsdk "github.com/openshift-online/ocm-sdk-go"
	acctrspv1 "github.com/openshift-online/ocm-sdk-go/accesstransparency/v1"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	jiraClient "github.com/openshift/backplane-cli/pkg/jira"
	"github.com/openshift/backplane-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func getJiraBaseURL() string {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		logger.Warnf("failed to load backplane config: %v, defaulting JIRA base URL to '%s'", err, config.JiraBaseURLDefaultValue)

		return config.JiraBaseURLDefaultValue
	}

	return bpConfig.JiraBaseURL
}

func GetClusterID(cmd *cobra.Command) (string, error) {
	clusterKey, err := cmd.Flags().GetString("cluster-id")
	if err != nil {
		return "", err
	}

	bpCluster, err := utils.DefaultClusterUtils.GetBackplaneCluster(clusterKey)
	if err != nil {
		return "", err
	}

	return bpCluster.ClusterID, nil
}

func GetAccessRequest(ocmConnection *ocmsdk.Connection, clusterID string) (*acctrspv1.AccessRequest, error) {
	isEnabled, err := ocm.DefaultOCMInterface.IsClusterAccessProtectionEnabled(ocmConnection, clusterID)
	if err != nil {
		return nil, fmt.Errorf("unable to determine if access protection is enabled or not for cluster '%s': %v", clusterID, err)
	}

	if !isEnabled {
		return nil, fmt.Errorf("access protection is not enabled for cluster '%s'", clusterID)
	}

	return ocm.DefaultOCMInterface.GetClusterActiveAccessRequest(ocmConnection, clusterID)
}

func PrintAccessRequest(clusterID string, accessRequest *acctrspv1.AccessRequest) {
	accessRequestStatus := accessRequest.Status()
	accessRequestStatusState := acctrspv1.AccessRequestState("<Undefined>")

	if accessRequestStatus != nil && accessRequestStatus.State() != "" {
		accessRequestStatusState = accessRequestStatus.State()
	}

	fmt.Printf("Active access request for cluster '%s':\n", clusterID)
	fmt.Printf("  Status                     : %s\n", accessRequestStatusState)

	switch accessRequestStatusState {
	case acctrspv1.AccessRequestStateApproved:
		fmt.Printf("  Approval expires at        : %s\n", accessRequestStatus.ExpiresAt())
	case acctrspv1.AccessRequestStatePending:
		fmt.Printf("  Expires at                 : %s\n", accessRequest.DeadlineAt())
		fmt.Printf("  Requested approval duration: %s\n", accessRequest.Duration())
	}
	fmt.Printf("  JIRA ticket used for notifs: %s/browse/%s\n", getJiraBaseURL(), accessRequest.InternalSupportCaseId())

	fmt.Printf("  Created by                 : %s\n", accessRequest.RequestedBy())
	fmt.Printf("  Reason/justification       : %s\n", accessRequest.Justification())
	fmt.Printf("  For more details, run      : ocm get %s\n", accessRequest.HREF())
}

func verifyAndPossiblyRetrieveIssue(bpConfig *config.BackplaneConfiguration, isProd bool, issueID string) (*jira.Issue, error) {
	issuesConfig := &bpConfig.JiraConfigForAccessRequests
	issueID = strings.TrimPrefix(issueID, getJiraBaseURL()+"/browse/")

	if isProd {
		if !strings.HasPrefix(issueID, issuesConfig.ProdProject+"-") {
			return nil, fmt.Errorf("issue does not belong to the %s prod JIRA project", issuesConfig.ProdProject)
		}
	} else {
		var knownProjects string
		var isIssueProjectValid = false

		for project := range issuesConfig.ProjectToTransitionsNames {
			if project != issuesConfig.ProdProject {
				if strings.HasPrefix(issueID, project+"-") {
					isIssueProjectValid = true
					break
				}
				if knownProjects != "" {
					knownProjects += ", "
				}
				knownProjects += project
			}
		}

		if !isIssueProjectValid {
			return nil, fmt.Errorf("issue does not belong to one of the '%s' test JIRA projects", knownProjects)
		}
	}

	if bpConfig.JiraToken == "" {
		logger.Warnf("won't verify the validity of the '%s' JIRA issue as no JIRA token is defined, consider defining it running 'ocm-backplane config set %s <token value>'", issueID, config.JiraTokenViperKey)

		return nil, nil
	}

	issue, _, err := jiraClient.DefaultIssueService.Get(issueID, nil)
	if err != nil {
		return nil, err
	}

	return issue, nil
}

func createNotificationIssue(bpConfig *config.BackplaneConfiguration, isProd bool, clusterID string) (*jira.Issue, error) {
	issuesConfig := &bpConfig.JiraConfigForAccessRequests
	issueProject := issuesConfig.DefaultProject
	issueType := issuesConfig.DefaultIssueType

	if isProd {
		issueProject = issuesConfig.ProdProject
		issueType = issuesConfig.ProdIssueType
	}

	issue := &jira.Issue{
		Fields: &jira.IssueFields{
			Description: "Access request tracker",
			Type: jira.IssueType{
				Name: issueType,
			},
			Project: jira.Project{
				Key: issueProject,
			},
			Summary: fmt.Sprintf("Access request tracker for cluster '%s'", clusterID),
		},
	}

	issue, _, err := jiraClient.DefaultIssueService.Create(issue)
	if err != nil {
		return nil, err
	}

	return issue, nil
}

func getProjectFromIssueID(issueID string) string {
	dashIdx := strings.Index(issueID, "-")

	if dashIdx < 0 {
		return ""
	}

	return issueID[0:dashIdx]
}

func transitionIssue(issueID, newTransitionName string) {
	possibleTransitions, _, err := jiraClient.DefaultIssueService.GetTransitions(issueID)
	if err != nil {
		logger.Warnf("won't transition the '%s' JIRA issue to '%s' as it was not possible to retrieve the possible transitions for the issue: %v", issueID, newTransitionName, err)
	} else {
		transitionID := ""

		for _, v := range possibleTransitions {
			if v.Name == newTransitionName {
				transitionID = v.ID
				break
			}
		}

		if transitionID == "" {
			logger.Warnf("won't transition the '%s' JIRA issue to '%s' as there is no transition named that way", issueID, newTransitionName)
		} else {
			_, err := jiraClient.DefaultIssueService.DoTransition(issueID, transitionID)

			if err != nil {
				logger.Warnf("failed to transition the '%s' JIRA issue to '%s': %v", issueID, newTransitionName, err)
			}
		}
	}
}

func updateNotificationIssueDescription(issue *jira.Issue, onApprovalTransitionName string, accessRequest *acctrspv1.AccessRequest) {
	issue.Fields = &jira.IssueFields{
		Description: fmt.Sprintf("Issue used for notifications purpose only.\n"+
			"Issue will moved in '%s' status when the customer approves the corresponding access request:\n%s",
			onApprovalTransitionName, accessRequest.HREF()),
	}

	_, _, err := jiraClient.DefaultIssueService.Update(issue)
	if err != nil {
		logger.Warnf("failed to update the description of the '%s' JIRA issue: %v", issue.Key, err)
	}
}

func CreateAccessRequest(ocmConnection *ocmsdk.Connection, clusterID, justification, notificationIssueID string, approvalDuration time.Duration) (*acctrspv1.AccessRequest, error) {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return nil, err
	}

	ocmEnv, err := ocm.DefaultOCMInterface.GetOCMEnvironment()
	if err != nil || ocmEnv == nil {
		return nil, err
	}
	isProd := ocmEnv.Name() == bpConfig.ProdEnvName

	var isOwningNotificationIssue = notificationIssueID == ""
	var notificationIssue *jira.Issue

	if isOwningNotificationIssue {
		notificationIssue, err = createNotificationIssue(&bpConfig, isProd, clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to create the notification issue, consider creating the issue separately and passing it with the --notification-issue option: %v", err)
		}
		notificationIssueID = notificationIssue.Key
	} else {
		notificationIssue, err = verifyAndPossiblyRetrieveIssue(&bpConfig, isProd, notificationIssueID)
		if err != nil {
			return nil, fmt.Errorf("issue '%s' passed with the --notification-issue option is invalid: %v", notificationIssueID, err)
		}
	}

	transitionsNames := bpConfig.JiraConfigForAccessRequests.ProjectToTransitionsNames[getProjectFromIssueID(notificationIssueID)]

	accessRequest, err := ocm.DefaultOCMInterface.CreateClusterAccessRequest(ocmConnection, clusterID, justification, notificationIssueID, approvalDuration.String())
	if err != nil {
		if isOwningNotificationIssue {
			transitionIssue(notificationIssueID, transitionsNames.OnError)
		}

		return nil, fmt.Errorf("failed to create a new access request for cluster '%s': %v", clusterID, err)
	}

	if notificationIssue != nil {
		updateNotificationIssueDescription(notificationIssue, transitionsNames.OnApproval, accessRequest)
		transitionIssue(notificationIssueID, transitionsNames.OnCreation)
	}

	return accessRequest, nil
}

func ExpireAccessRequest(ocmConnection *ocmsdk.Connection, accessRequest *acctrspv1.AccessRequest) error {
	_, err := ocm.DefaultOCMInterface.CreateAccessRequestDecision(ocmConnection, accessRequest, acctrspv1.DecisionDecisionExpired, "proactively expired using 'ocm-backplane accessrequest expire' CLI")
	if err != nil {
		return fmt.Errorf("failed to create a new decision for access request '%s': %v", accessRequest.HREF(), err)
	}

	return nil
}
