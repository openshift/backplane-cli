package logout

import (
	"fmt"
	"regexp"

	logger "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/backplane-cli/pkg/login"
	"github.com/openshift/backplane-cli/pkg/utils"
)

// LogoutCmd represents the logout command
var LogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout of the current cluster by deleting the related reference in kubeconfig",
	Long: `Logout command will remove the current kubeconfig context and
           remove the reference to the current cluster if you have logged on
           with backplane`,
	Example:      "ocm backplane logout",
	RunE:         runLogout,
	SilenceUsage: true,
}

func runLogout(cmd *cobra.Command, argv []string) error {

	// Logout specific cluster
	if len(argv) == 1 {
		clusterID, _, err := utils.DefaultOCMInterface.GetTargetCluster(argv[0])
		if err != nil {
			return err
		}

		// Remove cluster specific Kubeconfig file
		err = login.RemoveClusterKubeConfig(clusterID)
		if err != nil {
			return err
		}
		fmt.Printf("Logged out from backplane: %s\n", argv[0])
	} else {
		// Get raw kubeconfig from the default kubeconfig file
		rc, err := utils.ReadKubeconfigRaw()
		if err != nil {
			return err
		}

		// Kubeconfig has three main objects: Cluster/Context/User
		// Context is the current Cluster/User combination Kubeconfig is
		// currently working on

		// To cleanup, we use `CurrentContext` to obtain the cluster and user
		// and delete all relevant info
		currentContextObj := rc.Contexts[rc.CurrentContext]
		if currentContextObj == nil {
			return fmt.Errorf("current context does not exist, skipping")
		}
		currentUser := currentContextObj.AuthInfo
		currentCluster := currentContextObj.Cluster
		currentClusterObj := rc.Clusters[currentCluster]
		if currentClusterObj == nil {
			return fmt.Errorf("current cluster not found, skipping")
		}
		currentServer := currentClusterObj.Server

		// backplane should only handle `logout` associated context
		// created with backplane itself, we check this via matching
		// the cluster server endpoint
		backplaneServerRegex := regexp.MustCompile(utils.BackplaneAPIURLRegexp)

		logger.WithFields(logger.Fields{
			"currentServer":  currentServer,
			"currentUser":    currentUser,
			"currentContext": rc.CurrentContext,
		}).Debugln("Current context")

		if !backplaneServerRegex.MatchString(currentServer) {
			return fmt.Errorf("you're not logged in using backplane, skipping")
		}

		logger.Debugln("Logging out of the current cluster")

		// Delete the current cluster/context/user and set current-context to empty str
		delete(rc.Clusters, currentCluster)
		delete(rc.Contexts, rc.CurrentContext)
		delete(rc.AuthInfos, currentUser)
		savedContext := rc.CurrentContext
		// Setting current-context to empty str will make `oc` command return
		// errors saying that the config is incomplete, however, this is inline with
		// the behavior of `oc config unset current-context`
		rc.CurrentContext = ""

		pathOptions := clientcmd.NewDefaultPathOptions()
		err = clientcmd.ModifyConfig(pathOptions, rc, true)
		if err != nil {
			return err
		}
		logger.Debugln("Kubeconfig written")
		fmt.Printf("Logged out from backplane: %s\n", savedContext)
	}

	return nil
}
