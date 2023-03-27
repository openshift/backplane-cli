package login

import (
	"errors"
	"fmt"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	logger "github.com/sirupsen/logrus"
)

// Create cluster config based on cluster ID
func CreateClusterKubeConfig(clusterId string, kubeConfig api.Config) (string, error) {

	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create cluster folder
	path := homedir + "/.kube/" + clusterId
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", err
		}
	}

	// Write kube config if file not exist
	filename := path + "/" + "config"
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

	err = os.Setenv("KUBECONFIG", filename)
	if err != nil {
		return "", err
	}
	return filename, nil

}

// Delete
func RemoveClusterKubeConfig(clusterId string) error {

	homedir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	path := homedir + "/.kube/" + clusterId

	_, err = os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		os.RemoveAll(path)
	}
	return nil
}

// Save Kube config based on setting
func SaveKubeConfig(clusterId string, config api.Config, isMulti bool) error {

	if isMulti {
		//save config to current session
		path, err := CreateClusterKubeConfig(clusterId, config)
		fmt.Printf("Execute the following command to log into the cluster %s \n", clusterId)
		fmt.Println("export KUBECONFIG=" + path)

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
