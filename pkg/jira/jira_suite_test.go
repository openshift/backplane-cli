package jira_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJira(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jira Service Suite")
}
