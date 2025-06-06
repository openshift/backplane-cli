package announcements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandoverProjectService_ImplementsIssueSearcher(t *testing.T) {
	var _ issueSearcher = &HandoverProjectService{} // compile-time check

	// No runtime tests here because this depends on external Jira client.
	assert.True(t, true)
}
