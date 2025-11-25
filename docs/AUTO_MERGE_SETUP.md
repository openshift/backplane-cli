# Dependabot Auto-Merge Setup

This repository is configured to automatically merge safe dependency updates from Dependabot once they pass all CI checks.

## How It Works

### Dependabot Configuration
- **Location**: `.github/dependabot.yml`
- **Ecosystems**: Go modules (`gomod`) and Docker images
- **Schedule**: Weekly updates
- **Labels**: All Dependabot PRs are automatically labeled with `area/dependency` and `ok-to-test`
- **Grouping**: Related dependencies (AWS SDK, Kubernetes, OpenShift) are grouped together to reduce PR volume

### Auto-Merge Rules
The auto-merge workflow (`.github/workflows/dependabot-auto-merge.yml`) will automatically merge PRs that meet ALL of the following criteria:

✅ **Safe Update Types** (auto-merged):
- **Patch updates** (`1.2.3` → `1.2.4`) - Bug fixes and security patches
- **Minor updates** (`1.2.3` → `1.3.0`) - New features, backward compatible
- **Digest updates** - Docker image digest updates

❌ **Requires Manual Review** (NOT auto-merged):
- **Major updates** (`1.2.3` → `2.0.0`) - Potential breaking changes
- PRs missing required labels
- PRs that fail CI checks

### Required Labels
For auto-merge to work, Dependabot PRs must have these labels (automatically applied):
- `area/dependency`
- `ok-to-test`

## Testing the Setup

### Manual Testing
1. Create a test dependency update PR manually
2. Verify the auto-merge workflow triggers
3. Check that CI status checks are required
4. Confirm auto-merge only works for safe updates

### Monitoring Auto-Merge
- Check the "Actions" tab to see workflow runs
- Review auto-merge decisions in workflow logs
- Monitor Dependabot PRs for proper labeling and auto-merge behavior

## Troubleshooting

### Auto-Merge Not Working
1. **Check branch protection**: Ensure required status checks are configured
2. **Verify labels**: Dependabot PRs should have `area/dependency` and `ok-to-test`
3. **Review permissions**: GitHub Actions needs write permissions
4. **Check update type**: Only patch/minor/digest updates are auto-merged

### CI Failures
1. **Test failures**: Review test output in CI logs
2. **Lint failures**: Run `make lint` locally to fix issues
3. **Build failures**: Ensure code compiles with `make build`
4. **Security issues**: Review vulnerability scan results

### Manual Override
To manually merge a Dependabot PR that wasn't auto-merged:
1. Review the changes and changelog
2. Ensure all CI checks pass
3. Manually approve and merge the PR

## Security Considerations

- **Patch updates** are generally safe and contain security fixes
- **Minor updates** should be backward compatible but may introduce new features
- **Major updates** require manual review due to potential breaking changes
- **Vulnerability scanning** runs on all PRs to catch security issues
- **Branch protection** ensures no code is merged without passing CI

## Maintenance

### Regular Tasks
- Monitor auto-merge success rate
- Review any manually-merged dependency PRs for patterns
- Update CI workflows as needed
- Adjust Dependabot configuration based on project needs

### Updating This Setup
- Modify `.github/dependabot.yml` to change update frequency or add ignores
- Update `.github/workflows/dependabot-auto-merge.yml` to adjust auto-merge rules
- Modify `.github/workflows/ci.yml` to add or change CI checks
