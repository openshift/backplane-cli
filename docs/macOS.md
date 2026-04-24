# Running backplane-cli on macOS Apple Silicon

The `ocm backplane console` command runs the console container locally using the same container image as the logged-in cluster, which is typically built for Linux/amd64. For better compatibility and performance on macOS with Apple Silicon (M1/M2/M3), you should configure Podman to use `Rosetta` instead of `QEMU` for x86_64 emulation.

## Steps to Configure Podman with Rosetta

### 1. Update Podman

Ensure you have the latest version of Podman installed:

```bash
brew upgrade podman
```

Alternatively, download the latest installer from [podman.io](https://podman.io/).

### 2. Configure Rosetta Support

Edit `~/.config/containers/containers.conf` to specify the provider and enable Rosetta:

```ini
[machine]
provider = "applehv"
rosetta = true
```

### 3. Recreate the Podman Machine

Remove the existing VM and create a new one. **Warning:** This will erase all existing container images and data.

```bash
podman machine reset
podman machine init
podman machine start
```

### 4. Verify Rosetta is Enabled

Check that Rosetta is properly configured:

```bash
# Verify Rosetta is enabled in machine configuration
podman machine inspect --format '{{.Rosetta}}'
# Expected output: true

# Enable Rosetta activation
podman machine ssh "sudo touch /etc/containers/enable-rosetta"
podman machine ssh "sudo systemctl restart rosetta-activation.service"

# Verify Rosetta is available in binfmt
podman machine ssh "ls /proc/sys/fs/binfmt_misc/"
# Expected output should include 'rosetta' in the list
```
