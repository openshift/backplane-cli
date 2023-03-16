package console

// NOTE : This test will be fixed by OSD-15471

import (
	"testing"
	
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Console Test Suite")
}

