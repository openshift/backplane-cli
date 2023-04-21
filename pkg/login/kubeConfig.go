package login

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/backplane-cli/pkg/info"
	logger "github.com/sirupsen/logrus"
)

var (
	kubeConfigBasePath string
)

// CreateClusterKubeConfig creates cluster specific kube config based on a cluster ID
func CreateClusterKubeConfig(clusterId string, kubeConfig api.Config) (string, error) {

	basePath, err := getKubeConfigBasePath()
	if err != nil {
		return "", err
	}

	// Create cluster folder
	path := filepath.Join(basePath, clusterId)
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", err
		}
	}

	// Write kube config if file not exist
	filename := filepath.Join(path, "config")
	_, err = os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(filename)
		if err != nil {
			return "", err
		}
		err = clientcmd.WriteToFile(kubeConfig, f.Name())

		if err != nil {
			return "", err
		}
		err = f.Close()
		if err != nil {
			return "", err
		}
	}

	// set kube config env with temp kube config file

	err = os.Setenv(info.BACKPLANE_KUBECONFIG_ENV_NAME, filename)
	if err != nil {
		return "", err
	}
	return filename, nil

}

// RemoveClusterKubeConfig delete cluster specific kube config file
func RemoveClusterKubeConfig(clusterId string) error {

	basePath, err := getKubeConfigBasePath()
	if err != nil {
		return err
	}

	path := filepath.Join(basePath, clusterId)

	_, err = os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		os.RemoveAll(path)
	}
	return nil
}

// SaveKubeConfig modify Kube config based on user setting
func SaveKubeConfig(clusterId string, config api.Config, isMulti bool) error {

	if isMulti {
		//save config to current session
		path, err := CreateClusterKubeConfig(clusterId, config)
		fmt.Printf("Execute the following command to log into the cluster %s \n", clusterId)
		fmt.Println("export " + info.BACKPLANE_KUBECONFIG_ENV_NAME + "=" + path)

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
