package console

import (
	"fmt"
	"os"
)

// Mock Interfaces, due to the trivial nature of this mock, it doesn't warrent the use of gomock
type execActionOnTermMockStruct struct{}

// Immediately Terminate
func (e *execActionOnTermMockStruct) execActionOnTerminationFunction(action postTerminationAction) error {
	err := action()
	if err != nil {
		return err
	}
	return nil
}

// createPathPodman/Docker creates a fake path in the tmp directory to pretend the existence of a docker/podman binary
// The reason for this is that during testing there is no guarentee that podman/docker binaries are included
// Hence for the tests where we expect there to be podman/docker installed these helper functions could be used
// This should have no impact on the system other than the processes and children which
// runs this code
// It returns a string which is the path prior to the addition of the /tmp/tmp_bin directory
func createPathPodman() string {
	oldpath := os.Getenv("PATH")
	os.Setenv("PATH", oldpath+":/tmp/tmp_bin")
	err := os.MkdirAll("/tmp/tmp_bin", 0777)
	if err != nil {
		fmt.Printf("Failed to create the directory: %v\n", err)
	}
	pFile, err := os.CreateTemp("/tmp/tmp_bin", "")
	if err != nil {
		fmt.Printf("Failed to create the file: %v\n", err)
	}
	if err := os.Rename(pFile.Name(), "/tmp/tmp_bin/podman"); err != nil {
		fmt.Printf("Failed to rename the file: %v\n", err)
	}
	if err := os.Chmod("/tmp/tmp_bin/podman", 0777); err != nil {
		fmt.Printf("Failed to chmod the file: %v\n", err)
	}
	return oldpath
}

// See cretae path podman
// Essentially the same function but for a docker binary
func createPathDocker() string {
	oldpath := os.Getenv("PATH")
	os.Setenv("PATH", oldpath+":/tmp/tmp_bin")
	err := os.MkdirAll("/tmp/tmp_bin", 0777)
	if err != nil {
		fmt.Printf("Failed to create the directory: %v\n", err)
	}
	dFile, err := os.CreateTemp("/tmp/tmp_bin", "")
	if err != nil {
		fmt.Printf("Failed to create the file: %v\n", err)
	}
	if err := os.Rename(dFile.Name(), "/tmp/tmp_bin/docker"); err != nil {
		fmt.Printf("Failed to rename the file: %v\n", err)
	}
	if err := os.Chmod("/tmp/tmp_bin/docker", 0777); err != nil {
		fmt.Printf("Failed to chmod the file: %v\n", err)
	}
	return oldpath
}

func removePath(oldpath string) {
	os.Setenv("PATH", oldpath)
}
