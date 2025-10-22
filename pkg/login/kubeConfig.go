package login

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	logger "github.com/sirupsen/logrus"

	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
)

var (
	kubeConfigBasePath string
)

const (
	elevateExtensionName             = "ElevateContext"
	elevateExtensionRetentionMinutes = 20
)

type ElevateContext struct {
	Reasons  []string  `json:"reasons"`
	LastUsed time.Time `json:"lastUsed"`
}

///////////////////////////////////////////////////////////////////////
// runtime.Object interface func implementation for ElevateContext type

// DeepCopyObject creates a deep copy of the ElevateContext.
func (r *ElevateContext) DeepCopyObject() runtime.Object {
	return &ElevateContext{
		Reasons:  append([]string(nil), r.Reasons...),
		LastUsed: r.LastUsed,
	}
}

// GetObjectKind returns the schema.GroupVersionKind of the object.
func (r *ElevateContext) GetObjectKind() schema.ObjectKind {
	// return schema.EmptyObjectKind
	return &runtime.TypeMeta{
		Kind:       "ElevateContext",
		APIVersion: "v1",
	}
}

///////////////////////////////////////////////////////////////////////

// CreateClusterKubeConfig creates cluster specific kube config based on a cluster ID
func CreateClusterKubeConfig(clusterID string, kubeConfig api.Config) (string, error) {

	basePath, err := getKubeConfigBasePath()
	if err != nil {
		return "", err
	}

	// Create cluster folder
	path := filepath.Join(basePath, clusterID)
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm) //nolint:gosec
		if err != nil {
			return "", err
		}
	}

	// Write kube config
	filename := filepath.Join(path, "config")
	f, err := os.Create(filename) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	err = clientcmd.WriteToFile(kubeConfig, f.Name())
	if err != nil {
		return "", err
	}

	// set kube config env with temp kube config file
	err = os.Setenv(info.BackplaneKubeconfigEnvName, filename)
	if err != nil {
		return "", err
	}
	return filename, nil

}

// RemoveClusterKubeConfig delete cluster specific kube config file
func RemoveClusterKubeConfig(clusterID string) error {

	basePath, err := getKubeConfigBasePath()
	if err != nil {
		return err
	}

	path := filepath.Join(basePath, clusterID)

	_, err = os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(path)
	}
	return nil
}

// SaveKubeConfig modify Kube config based on user setting
func SaveKubeConfig(clusterID string, config api.Config, isMulti bool, kubePath string) error {

	if isMulti {
		//update path
		if kubePath != "" {
			err := SetKubeConfigBasePath(kubePath)
			if err != nil {
				return err
			}
		}
		//save config to current session
		path, err := CreateClusterKubeConfig(clusterID, config)

		if kubePath == "" {
			// Inform how to setup kube config
			fmt.Printf("# Execute the following command to log into the cluster %s \n", clusterID)
			fmt.Println("export " + info.BackplaneKubeconfigEnvName + "=" + path)
		}

		if err != nil {
			return err
		}

	} else {
		// Save the config to default path.
		configAccess := clientcmd.NewDefaultPathOptions()
		err := clientcmd.ModifyConfig(configAccess, config, true)

		if err != nil {
			return err
		}
	}
	logger.Debugln("Wrote Kube configuration")
	conf, _ := json.Marshal(config)
	logger.Debugln(string(conf))
	return nil
}

func getKubeConfigBasePath() (string, error) {
	if kubeConfigBasePath == "" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		kubeConfigBasePath = filepath.Join(homedir, ".kube")

		return kubeConfigBasePath, nil
	}

	return kubeConfigBasePath, nil
}

func SetKubeConfigBasePath(basePath string) error {
	kubeConfigBasePath = basePath
	return nil
}

// in some cases (mainly when config is created from json) the "ElevateContext Extension" is created as runtime.Unknow object
// instead of the desired ElevateContext, so we need to Unmarshal the raw definition in that case
func GetElevateContextReasons(config api.Config) []string {
	if currentContext := config.Contexts[config.CurrentContext]; currentContext != nil {
		var elevateContext *ElevateContext
		var ok bool
		if object := currentContext.Extensions[elevateExtensionName]; object != nil {
			//Let's first try to cast the extension object in ElevateContext
			if elevateContext, ok = object.(*ElevateContext); !ok {
				// and if it does not work, let's try cast the extension object in Unknown
				if unknownObject, ok := object.(*runtime.Unknown); ok {
					// and unmarshal the unknown raw JSON string into the ElevateContext
					_ = json.Unmarshal([]byte(unknownObject.Raw), &elevateContext)
				}
			}
			// We should keep the stored ElevateContext only if it is still valid
			if elevateContext != nil && time.Since(elevateContext.LastUsed) <= elevateExtensionRetentionMinutes*time.Minute {
				return elevateContext.Reasons
			}
		}
	}
	return []string{}
}

func AddElevationReasonsToRawKubeconfig(config api.Config, elevationReasons []string) error {
	logger.Debugln("Adding reason for backplane-cluster-admin elevation")
	if config.Contexts[config.CurrentContext] == nil {
		return errors.New("no current kubeconfig context")
	}

	currentCtxUsername := config.Contexts[config.CurrentContext].AuthInfo

	if config.AuthInfos[currentCtxUsername] == nil {
		return errors.New("no current user information")
	}

	if config.AuthInfos[currentCtxUsername].ImpersonateUserExtra == nil {
		config.AuthInfos[currentCtxUsername].ImpersonateUserExtra = make(map[string][]string)
	}

	config.AuthInfos[currentCtxUsername].ImpersonateUserExtra["reason"] = elevationReasons
	config.AuthInfos[currentCtxUsername].Impersonate = "backplane-cluster-admin"

	return nil
}

func SaveElevateContextReasons(config api.Config, elevationReason string) ([]string, error) {
	currentCtx := config.Contexts[config.CurrentContext]
	if currentCtx == nil {
		return nil, errors.New("no current kubeconfig context")
	}

	// let's first retrieve previous elevateContext if any, and add any provided reason.
	elevationReasons := utils.AppendUniqNoneEmptyString(
		GetElevateContextReasons(config),
		elevationReason,
	)

	// if we still do not have reason, then let's try to have the reason from prompt
	if len(elevationReasons) == 0 {
		elevationReasons = utils.AppendUniqNoneEmptyString(
			elevationReasons,
			utils.AskQuestionFromPrompt(fmt.Sprintf("Please enter a reason for elevation, it will be stored in current context for %d minutes : ", elevateExtensionRetentionMinutes)),
		)
	}
	// and raise an error if not possible
	if len(elevationReasons) == 0 {
		return nil, errors.New("please enter a reason for elevation")
	}

	// Store the ElevateContext in config current context Extensions map
	if currentCtx.Extensions == nil {
		currentCtx.Extensions = map[string]runtime.Object{}
	}
	currentCtx.Extensions[elevateExtensionName] = &ElevateContext{
		Reasons:  elevationReasons,
		LastUsed: time.Now(),
	}

	// Save the config to default path.
	configAccess := clientcmd.NewDefaultPathOptions()
	err := clientcmd.ModifyConfig(configAccess, config, true)

	return elevationReasons, err
}
