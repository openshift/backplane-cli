<!--
PR Title Format (please follow):

[Ticket(optional)] type: short description

Examples:
[SREP-1234] feat: add gcp cloud support
fix: fix timezone convertion
-->

### What type of PR is this?

- [ ] fix (Bug Fix)
- [ ] feat (New Feature)
- [ ] docs (Documentation)
- [ ] test (Test Coverage)
- [ ] chore (Clean Up / Maintenance Tasks)
- [ ] other (Anything that doesn't fit the above)

### What this PR does / Why we need it?

### Which Jira/Github issue(s) does this PR fix?

- Related Issue #
- Closes #

### Special notes for your reviewer

### Unit Test Coverage
#### Guidelines
- If it's a new sub-command or new function to an existing sub-command, please cover at least 50% of the code
- If it's a bug fix for an existing sub-command, please cover 70% of the code 
 
#### Test coverage checks  
- [ ] Added unit tests
- [ ] Created jira card to add unit test
- [ ] This PR may not need unit tests

### Pre-checks (if applicable)
- [ ] Ran unit tests locally
- [ ] Validated the changes in a cluster
- [ ] Included documentation changes with PR
- [ ] Backward compatible

<!-- Keep the below label to auto squash commits -->
/label tide/merge-method-squash
