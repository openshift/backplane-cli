package session

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/openshift/backplane-cli/cmd/ocm-backplane/login"
	"github.com/openshift/backplane-cli/pkg/cli/config"
	"github.com/openshift/backplane-cli/pkg/info"
	"github.com/openshift/backplane-cli/pkg/utils"
	"github.com/spf13/cobra"
)

// BackplaneSessionInterface abstract backplane session functions
type BackplaneSessionInterface interface {
	RunCommand(cmd *cobra.Command, args []string) error
	Setup() error
	Start() error
	Delete() error
}

// BackplaneSession struct for default Backplane session
type BackplaneSession struct {
	Path    string
	Exists  bool
	Options *Options
}

// Options define deafult backplane session options
type Options struct {
	DeleteSession bool

	Alias string

	ClusterId   string
	ClusterName string
}

var (
	DefaultBackplaneSession BackplaneSessionInterface = &BackplaneSession{}
)

// RunCommand setup session and allows to execute commands
func (e *BackplaneSession) RunCommand(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		e.Options.Alias = args[0]
	}
	if e.Options.ClusterId == "" && e.Options.Alias == "" {
		err := cmd.Help()
		if err != nil {
			return fmt.Errorf("could not print help")
		}
		return fmt.Errorf("ClusterId or Alias required")
	}

	if e.Options.Alias == "" {
		log.Println("No Alias set, using cluster ID")
		e.Options.Alias = e.Options.ClusterId
	}

	sessionPath, err := e.sessionPath()
	if err != nil {
		return fmt.Errorf("could not init session path")
	}
	e.Path = sessionPath

	if e.Options.DeleteSession {
		fmt.Printf("Cleaning up Backplane session %s\n", e.Options.Alias)
		err = e.Delete()
		if err != nil {
			return fmt.Errorf("could not delete the session. error: %v", err)
		}
		return nil
	}

	err = e.Setup()
	if err != nil {
		return fmt.Errorf("could not setup session. error: %v", err)
	}

	// Init cluster login via cluster ID or Alias
	err = e.initClusterLogin(cmd)
	if err != nil {
		return fmt.Errorf("could not login to the cluster. error: %v", err)
	}

	err = e.Start()
	if err != nil {
		return fmt.Errorf("could not start session. error: %v", err)
	}
	return nil
}

// Setup intitate the sessoion environment
func (e *BackplaneSession) Setup() error {
	// Delete session if exist
	err := e.Delete()
	if err != nil {
		return fmt.Errorf("error deleting session. error: %v", err)
	}

	// Setup clusterID and clusterName
	clusterKey := e.Options.Alias
	if e.Options.ClusterId != "" {
		clusterKey = e.Options.ClusterId
	}

	clusterId, clusterName, err := utils.DefaultOCMInterface.GetTargetCluster(clusterKey)

	if err == nil {
		// set cluster options
		e.Options.ClusterName = clusterName
		e.Options.ClusterId = clusterId
	}

	err = e.ensureEnvDir()
	if err != nil {
		return fmt.Errorf("error validating env directory. error: %v", err)
	}

	e.printSesionHeader()

	// Create session Bins
	err = e.createBins()
	if err != nil {
		return fmt.Errorf("error creating bins. error: %v", err)
	}

	// Creating history files
	err = e.createHistoryFile()
	if err != nil {
		return fmt.Errorf("error creating history files. error: %v", err)
	}

	// Validaing env variables
	err = e.ensureEnvVariables()
	if err != nil {
		return fmt.Errorf("error setting env vars. error: %v", err)
	}

	return nil
}

// Start trigger the session start
func (e *BackplaneSession) Start() error {
	shell := os.Getenv("SHELL")

	fmt.Print("Switching to Backplane session " + e.Options.Alias + "\n")
	cmd := exec.Command(shell)

	path := filepath.Clean(e.Path + "/.ocenv")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Println("Error closing file: ", path)
			return
		}
	}()
	scanner := bufio.NewScanner(file)
	cmd.Env = os.Environ()
	for scanner.Scan() {
		line := scanner.Text()
		cmd.Env = append(cmd.Env, line)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = e.Path
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error while running cmd. error %v", err)
	}

	err = e.killChildren()
	if err != nil {
		return err
	}

	fmt.Printf("Exited Backplane session \n")

	return nil
}

// killChildren delete all pds in .killpds file
func (e *BackplaneSession) killChildren() error {
	path := filepath.Join(e.Path, "/.killpds")

	if _, err := os.Stat(path); err == nil {
		file, err := os.Open(path)

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Println("Nothing to kill")
			}
		}
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				fmt.Println("Error while closing file: ", path)
				return
			}
		}(file)

		scanner := bufio.NewScanner(file)

		scanner.Split(bufio.ScanLines)
		var text []string

		for scanner.Scan() {
			text = append(text, scanner.Text())
		}

		for _, pid := range text {
			fmt.Printf("Stopping process %s\n", pid)
			pidNum, err := strconv.Atoi(pid)
			if err != nil {
				return fmt.Errorf("failed to read PID %s, you may need to clean up manually: %v", pid, err)
			}
			err = syscall.Kill(pidNum, syscall.SIGTERM)
			if err != nil {
				return fmt.Errorf("failed to stop child processes %s, you may need to clean up manually: %v", pid, err)
			}
		}

		err = os.Remove(path)
		if err != nil {
			return fmt.Errorf("failed to delete .killpids, you may need to clean it up manually: %v", err)
		}
	}

	return nil
}

// Delete cleanup the backplane session
func (e *BackplaneSession) Delete() error {
	err := os.RemoveAll(e.Path)
	if err != nil {
		fmt.Println("error while calling os.RemoveAll", err.Error())

	}
	return nil
}

// ensureEnvDir create session dirs if it's not exist
func (e *BackplaneSession) ensureEnvDir() error {
	if _, err := os.Stat(e.Path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(e.Path, os.ModePerm)
		if err != nil {
			return err
		}
	}
	e.Exists = true
	return nil
}

// ensureEnvDir intiate session env vars
func (e *BackplaneSession) ensureEnvVariables() error {
	envContent := `
HISTFILE=` + e.Path + `/.history
PATH=` + e.Path + `/bin:` + os.Getenv("PATH") + `
`

	if e.Options.ClusterId != "" {
		clusterEnvContent := "KUBECONFIG=" + filepath.Join(e.Path, e.Options.ClusterId, "config") + "\n"
		clusterEnvContent = clusterEnvContent + "CLUSTERID=" + e.Options.ClusterId + "\n"
		clusterEnvContent = clusterEnvContent + "CLUSTERNAME=" + e.Options.ClusterName + "\n"
		envContent = envContent + clusterEnvContent
	}
	direnvfile, err := e.ensureFile(e.Path + "/.ocenv")
	if err != nil {
		return err
	}
	_, err = direnvfile.WriteString(envContent)
	if err != nil {
		log.Fatal(err)
	}
	defer func(direnvfile *os.File) {
		direnvfile.Close()
	}(direnvfile)

	zshenvfile, err := e.ensureFile(e.Path + "/.zshenv")
	if err != nil {
		return err
	}
	_, err = zshenvfile.WriteString("source .ocenv")
	if err != nil {
		log.Fatal(err)
	}
	defer func(direnvfile *os.File) {
		err := direnvfile.Close()
		if err != nil {
			fmt.Println("Error while calling direnvFile.Close(): ", err.Error())
			return
		}
	}(direnvfile)
	return nil
}

func (e *BackplaneSession) createHistoryFile() error {
	historyFile := filepath.Join(e.Path, "/.history")
	scriptfile, err := e.ensureFile(historyFile)
	if err != nil {
		return err
	}
	defer func(scriptfile *os.File) {
		err := scriptfile.Close()
		if err != nil {
			fmt.Println("Error closing file: ", historyFile)
			return
		}
	}(scriptfile)
	return nil
}

// createBins create bins inside the session folder bin dir
func (e *BackplaneSession) createBins() error {
	if _, err := os.Stat(e.binPath()); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(e.binPath(), os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}
	err := e.createBin("ocd", "ocm describe cluster "+e.Options.ClusterId)
	if err != nil {
		return err
	}
	ocb := `
#!/bin/bash

set -euo pipefail

`
	err = e.createBin("ocb", ocb)
	if err != nil {
		return err
	}
	return nil
}

// createBin create bin file with given content
func (e *BackplaneSession) createBin(cmd string, content string) error {
	path := filepath.Join(e.binPath(), cmd)
	scriptfile, err := e.ensureFile(path)
	if err != nil {
		return err
	}
	defer func(scriptfile *os.File) {
		err := scriptfile.Close()
		if err != nil {
			fmt.Println("Error closing file: ", path)
			return
		}
	}(scriptfile)
	_, err = scriptfile.WriteString(content)
	if err != nil {
		return fmt.Errorf("error writing to file %s: %v", path, err)
	}
	err = os.Chmod(path, 0700)
	if err != nil {
		return fmt.Errorf("can't update permissions on file %s: %v", path, err)
	}
	return nil
}

// ensureFile check the existance of file in session path
func (e *BackplaneSession) ensureFile(filename string) (file *os.File, err error) {
	filename = filepath.Clean(filename)
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		file, err = os.Create(filename)
		if err != nil {
			return nil, fmt.Errorf("can't create file %s: %v", filename, err)
		}
	}
	return file, nil
}

// binPath returns the session bin path
func (e *BackplaneSession) binPath() string {
	return e.Path + "/bin"
}

// sessionPath returns the session saving path
func (e *BackplaneSession) sessionPath() (string, error) {
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return "", err
	}
	sessionDir := info.BACKPLANE_DEFAULT_SESSION_DIRECTORY

	// Get the session directory name via config
	if bpConfig.SessionDirectory != "" {
		sessionDir = bpConfig.SessionDirectory
	}

	UserHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configFilePath := filepath.Join(UserHomeDir, sessionDir, e.Options.Alias)
	return configFilePath, nil
}

// initCluster login to cluster and save kube config into session for valid clusters
func (e *BackplaneSession) initClusterLogin(cmd *cobra.Command) error {

	if e.Options.ClusterId != "" {

		// Setting up the flags
		err := login.LoginCmd.Flags().Set("multi", "true")
		if err != nil {
			return fmt.Errorf("error occered when setting multi flag %v", err)
		}
		err = login.LoginCmd.Flags().Set("kube-path", e.Path)
		if err != nil {
			return fmt.Errorf("error occered when kube-path flag %v", err)
		}

		// Execute login command
		err = login.LoginCmd.RunE(cmd, []string{e.Options.ClusterId})
		if err != nil {
			return fmt.Errorf("error occered when login to the cluster %v", err)
		}
	}

	return nil
}

func (e *BackplaneSession) printSesionHeader() {
	fmt.Println("====================================================")
	fmt.Println("*          Backplane Session                       *")
	fmt.Println("*                                                  *")
	fmt.Println("*Help:                                             *")
	fmt.Println("* exit will terminate this session                 *")
	fmt.Println("* You can use oc commands to interact with cluster *")
	fmt.Println("====================================================")
}
