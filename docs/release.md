# How to generate a new release

This document describes how to generate a new release for backplane-cli.

**Note:** Only maintainers or owners of this repository can perform the below steps.

### GitHub Token

To release to GitHub, you'll need to export a `GITHUB_TOKEN` environment variable, which should contain a valid GitHub token with the repo scope.

It will be used to deploy releases to your GitHub repository. You can create a new GitHub token [here](https://github.com/settings/tokens/new).

- Pick a name and a reasonable expiration date (1 day should be enough).
- Grant the `write:packages` permission.
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

### Determine a new version
backplane-cli follows [semver](https://semver.org/). The version is in format `v[Major].[Minor].[Patch]`.

- MAJOR version when you make incompatible API changes
- MINOR version when you add functionality in a backward compatible manner
- PATCH version when you make backward compatible bug fixes

Review the commits since last release:
```
$ git fetch upstream

$ git log $(git describe --tags --abbrev=0 upstream/main)..upstream/main --pretty=format:"%h %s" |
gawk '
{
  # Extract the commit message without the hash and ticket prefix
  # Pattern: hash + space + [ticket] + space + type + ":"
  # Example: "a1b2c3d [PROJ-123] feat: some message"

  # Remove the hash and space
  msg = substr($0, index($0,$2))

  # Regex to extract type: after "] " followed by letters, colon
  if (match(msg, /\] *([a-z]+):/, m)) {
    type = m[1]
    groups[type] = groups[type] "\n- " $0
  } else {
    groups["others"] = groups["others"] "\n- " $0
  }
}
END {
  order = "feat fix chore docs test others"
  split(order, o)
  for (i in o) {
    t = o[i]
    if (t in groups) {
      print "## " toupper(substr(t,1,1)) substr(t,2)
      print groups[t] "\n"
    }
  }
}
'  > /tmp/release-note.md
```
This saves the grouped commits in `/tmp/release-note.md`, please review and modify the release note file.

Increase `Major` when:
- It has changes that breaks backward compatibility:
    - For example, refactoring the CLI which changes the subcommand and argument format.
    - The user need to use a new command to perform a task which the old command no longer works.
- Reset `Minor` and `Patch` to 0.

Increase `Minor` when:
- It adds a new feature:
    - For example, adding a new subcommand, adding a new functionality to a subcommand.
    - The same commands in the old version can still work in the new version.
- Keep `Major` unchanged, and reset `Patch` to 0.

Increase `Patch` when:
- It has a bug fix that doesn't change the expected behaviors of a subcommand.
- It has dependency updates.
- Keep `Major` and `Minor` unchanged.

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
make release-with-note NOTE=/tmp/release-note.md
```

A new release will show up in the [releases](https://github.com/openshift/backplane-cli/releases) page.
