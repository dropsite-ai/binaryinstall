# binaryinstall

Install an already-copied compressed archive binary onto a remote server.

## Introduction

This Go package and CLI installs a tar archive compressed with gzip that was already copied onto the remote server. It places the binary into the correct final location (e.g., /usr/local/bin) with correct permissions, backups, etc.

## Installation

### Go Package

```bash
go get github.com/dropsite-ai/binaryinstall
```

### Homebrew (macOS or Compatible)

If you use Homebrew, install binaryinstall with:
```bash
brew tap dropsite-ai/homebrew-tap
brew install binaryinstall
```

### Download Binaries

Grab the latest pre-built binaries from the [GitHub Releases](https://github.com/dropsite-ai/binaryinstall/releases). Extract them, then run the `binaryinstall` executable directly.

### Build from Source

1. **Clone the repository**:
   ```bash
   git clone https://github.com/dropsite-ai/binaryinstall.git
   cd binaryinstall
   ```
2. **Build using Go**:
   ```bash
   go build -o binaryinstall cmd/main.go
   ```

## Usage

### Go Package Usage

You can import the package into your Go project and call its API directly. Instead of a plain string slice, each “upload” can now specify `Path`, `DestinationDir`, `Owner`, `Permission`, and a boolean `BindLowPorts`.

Example:

```go
package main

import (
    "log"

    "github.com/dropsite-ai/binaryinstall"
)

func main() {
    config := binaryinstall.BinaryInstallConfig{
        RemoteHost: "ec2-12-34-56-78.compute-1.amazonaws.com",
        SSHUser:    "ec2-user",
        SSHKeyPath: "/path/to/ssh-key.pem",
        Uploads: []binaryinstall.BinaryUpload{
            {
                Path:          "/home/ec2-user/llmfs_Darwin_arm64.tar.gz",
                DestinationDir: "/usr/local/bin",
                Owner:         "root",
                Permission:    "0755",
                BindLowPorts:  false,
            },
            {
                Path:          "/home/ec2-user/llmfs_Linux_x86_64.tar.gz",
                DestinationDir: "/usr/local/bin",
                Owner:         "root",
                Permission:    "0755",
                BindLowPorts:  true, // triggers setcap
            },
        },
        BackupDir: "/home/ec2-user/bin.old",
        Verbose:   true,
    }

    if err := binaryinstall.InstallBinaries(config); err != nil {
        log.Fatalf("Installation failed: %v", err)
    }
    log.Println("Binaries installed successfully.")
}
```

### CLI Usage

After building or installing the `binaryinstall` CLI, run it from your terminal. Use the `-upload` flag **once per upload**, with a comma-delimited string to specify:

- **path**: Full path to the tar.gz on the remote.
- **dest**: Destination directory for the installed binary.
- **owner**: Owner user/group.
- **perm**: Permission string (e.g. 0755).
- **bindlowports**: `true` or `false` if the binary needs `cap_net_bind_service`.

For example:

```bash
./binaryinstall \
  -remote ec2-12-34-56-78.compute-1.amazonaws.com \
  -sshuser ec2-user \
  -sshkey /path/to/ssh-key.pem \
  -upload "path=/home/ec2-user/llmfs_Darwin_arm64.tar.gz,dest=/usr/local/bin,owner=root,perm=0755,bindlowports=false" \
  -upload "path=/home/ec2-user/llmfs_Linux_x86_64.tar.gz,dest=/usr/local/bin,owner=root,perm=0755,bindlowports=true" \
  -backup /home/ec2-user/bin.old \
  -verbose
```

This command will:
- Connect to the remote host via SSH.
- Process each `-upload` tar.gz archive.  
- Derive the final binary name by stripping `.tar.gz` and everything after the first underscore (e.g. `llmfs_Linux_x86_64.tar.gz` → `llmfs`).
- Place the binary in `/usr/local/bin` and back up any old version to `/home/ec2-user/bin.old`.
- Apply the correct owner (`root`) and permissions (`0755`).
- **If** an entry has `bindlowports=true`, run `sudo setcap 'cap_net_bind_service=+ep'` on the installed binary so it can listen on ports < 1024.
- Show detailed command logs if `-verbose` is set.

## Test

```bash
make test
```

## Release

```bash
make release
```