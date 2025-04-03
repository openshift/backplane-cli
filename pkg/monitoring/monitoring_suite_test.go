package monitoring

import (
	"io"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Monitoring Test Suite")
}

func MakeIoReader(s string) io.ReadCloser {
	r := io.NopCloser(strings.NewReader(s))
	return r
}
