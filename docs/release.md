# How to generate a new release

This document describes how to generate a new release for backplane-cli.

**Note:** Only maintainers or owners of this repository can perform the below steps.

### GitHub Token

To release to GitHub, you'll need to export a `GITHUB_TOKEN` environment variable, which should contain a valid GitHub token with the repo scope.

It will be used to deploy releases to your GitHub repository. You can create a new GitHub token [here](https://github.com/settings/tokens/new).

- Pick a name and a reasonable expiration date (1 day should be enough).
- Grant API and write_repository permissions.
- Export the token for later use:

```bash
export GITHUB_TOKEN="YOUR_GH_TOKEN"
```

### Local repository setup

Fork `openshift/backplane-cli` and add the git upstream.

```bash
git clone <your-fork>
cd backplane-cli
git remote add upstream https://github.com/openshift/backplane-cli.git
```

### Cutting a new release

Create a tag on the latest main.

```bash
git fetch upstream
git checkout upstream/main
git tag -a ${VERSION} -m "release ${VERSION}"
git push upstream $VERSION
```

**Note:** We follow [semver](https://semver.org/) for versioning. Release tags are expected to be suffixed with a `v` for consistent naming; For example, `v1.0.0`.

Run goreleaser to build the binaries and create the release page.

```bash
git checkout upstream/main
make release
```

A new release will show up in the [releases](https://github.com/openshift/backplane-cli/releases) page.
