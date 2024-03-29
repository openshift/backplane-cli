package session

import (
	"io"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Session Test Suite")
}

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s))
	return r
}
