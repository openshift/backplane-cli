package login

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	logger "github.com/sirupsen/logrus"

	"github.com/openshift/backplane-cli/pkg/info"
)

var (
	kubeConfigBasePath string
)

// CreateClusterKubeConfig creates cluster specific kube config based on a cluster ID
func CreateClusterKubeConfig(clusterID string, kubeConfig api.Config) (string, error) {

	basePath, err := getKubeConfigBasePath()
	if err != nil {
		return "", err
	}

	// Create cluster folder
	path := filepath.Join(basePath, clusterID)
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", err
		}
	}

	// Write kube config
	filename := filepath.Join(path, "config")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		f.Close()
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
		os.RemoveAll(path)
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
