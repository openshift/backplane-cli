# Frequently Asked Questions

## Why is the binary named ocm-backplane?
With the name `ocm-*`, the binary can act as an [OCM plugin](https://github.com/openshift-online/ocm-cli/blob/119e9d2e8af0ddf5db4b0888b5ceadb300768885/README.md#extend-ocm-with-plugins).

The logic behind OCM plugin:
- The user runs `ocm backplane`.
- `ocm` doesn't have the `backplane` subcommand.
- `ocm` looks up the `$PATH` and finds the `ocm-backplane` executable binary.
- `ocm` calls the `ocm-backplane` binary.

## Does ocm-backplane depend on a specific OCM binary version?
`ocm-backplane` is a standalone binary. It is functional even without the `ocm` binaries. The interaction with OCM API will be handled by OCM SDK, which is defined as a dependency of the project.
