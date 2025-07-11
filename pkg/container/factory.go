package container

import (
	"fmt"
	"os"
	"path/filepath"
)

func NewEngine(osName, containerEngine string) (ContainerEngine, error) {
	if osName == LINUX && containerEngine == PODMAN {
		return &podmanLinux{fileMountDir: filepath.Join(os.TempDir(), "backplane")}, nil
	} else if osName == MACOS && containerEngine == PODMAN {
		return &podmanMac{}, nil
	} else if osName == LINUX && containerEngine == DOCKER {
		return &dockerLinux{}, nil
	} else if osName == MACOS && containerEngine == DOCKER {
		return &dockerMac{}, nil
	} else {
		return nil, fmt.Errorf("unsupported container engine: %s/%s", osName, containerEngine)
	}
}
