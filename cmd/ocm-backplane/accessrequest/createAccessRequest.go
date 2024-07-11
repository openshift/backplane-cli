package accessrequest

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/backplane-cli/pkg/accessrequest"

	ocmcli "github.com/openshift-online/ocm-cli/pkg/ocm"
	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/utils"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	options struct {
		reason              string
		notificationIssueID string
		pendingDuration     time.Duration
		approvalDuration    time.Duration
	}
)

// newCreateAccessRequestCmd returns cobra command
func newCreateAccessRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "create",
		Short:         "Creates a new pending access request",
		Args:          cobra.ExactArgs(0),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runCreateAccessRequest,
	}

	cmd.Flags().StringVarP(
		&options.reason,
		"reason",
		"r",
		"",
		"Reason/justification passed through the access request to the customer. "+
			"Reason will be read from the kube context (unless --cluster-id is set) or prompted if the option is not set.")

	cmd.Flags().StringVarP(
		&options.notificationIssueID,
		"notification-issue",
		"n",
		"",
		"JIRA issue used for notifications when the access request is approved or denied. "+
			"Issue needs to belong to the OHSS project on production and to the SDAINT project for staging & integration. "+
			"Issue will automatically be created in the proper project if the option is not set.")

	cmd.Flags().DurationVarP(
		&options.approvalDuration,
		"approval-duration",
		"d",
		8*time.Hour,
		"The maximal period of time during which the access request can stay approved")

	return cmd
}

func retrieveOrPromptReason(cmd *cobra.Command) string {
	if utils.CheckValidPrompt() {
		clusterKey, err := cmd.Flags().GetString("cluster-id")

		if err == nil && clusterKey == "" {
			config, err := utils.ReadKubeconfigRaw()

			if err == nil {
				reasons := login.GetElevateContextReasons(config)
				for _, reason := range reasons {
					if reason != "" {
						fmt.Printf("Reason for elevations read from the kube config: %s\n", reason)
						if strings.ToLower(utils.AskQuestionFromPrompt("Do you want to use this as the reason/justification for the access request to create (Y/n)? ")) != "n" {
							return reason
						}
						break
					}
				}
			} else {
				logger.Warnf("won't extract the elevation reason from the kube context which failed to be read: %v", err)
			}
		}
	}

	return utils.AskQuestionFromPrompt("Please enter a reason/justification for the access request to create: ")
}

// runCreateAccessRequest creates access request for the given cluster
func runCreateAccessRequest(cmd *cobra.Command, args []string) error {
	clusterID, err := accessrequest.GetClusterID(cmd)
	if err != nil {
		return fmt.Errorf("failed to compute cluster ID: %v", err)
	}

	ocmConnection, err := ocmcli.NewConnection().Build()
	if err != nil {
		return fmt.Errorf("failed to create OCM connection: %v", err)
	}
	defer ocmConnection.Close()

	accessRequest, err := accessrequest.GetAccessRequest(ocmConnection, clusterID)

	if err != nil {
		return err
	}

	if accessRequest != nil {
		accessrequest.PrintAccessRequest(clusterID, accessRequest)

		return fmt.Errorf("there is already an active access request for cluster '%s', eventually consider expiring it running 'ocm-backplane accessrequest expire'", clusterID)
	}

	reason := options.reason
	if reason == "" {
		reason = retrieveOrPromptReason(cmd)
		if reason == "" {
			return errors.New("no reason/justification, consider using the --reason option with a non empty string")
		}
	}

	accessRequest, err = accessrequest.CreateAccessRequest(ocmConnection, clusterID, reason, options.notificationIssueID, options.approvalDuration)

	if err != nil {
		return err
	}

	accessrequest.PrintAccessRequest(clusterID, accessRequest)

	return nil
}
