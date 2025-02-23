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

You can import the package into your Go project and call its API directly. The binary name is automatically derived from each archive's filename by stripping the `.tar.gz` extension and taking the substring before the first underscore. For example, consider the following code:

```go
package main

import (
	"log"

	"github.com/dropsite-ai/binaryinstall"
)

func main() {
	config := binaryinstall.BinaryInstallConfig{
		RemoteHost:     "ec2-12-34-56-78.compute-1.amazonaws.com",
		SSHUser:        "ec2-user",
		SSHKeyPath:     "/path/to/ssh-key.pem",
		// Specify one or more archive paths.
		// The uncompressed binary name will be derived automatically from each filename.
		UploadPaths: []string{
			"/home/ec2-user/llmfs_Darwin_arm64.tar.gz",
			"/home/ec2-user/llmfs_Linux_x86_64.tar.gz",
		},
		DestinationDir: "/usr/local/bin",
		BackupDir:      "/home/ec2-user/bin.old",
		Owner:          "root",
		Permission:     "0755",
		Verbose:        true, // Enable verbose output for detailed command logging.
	}

	if err := binaryinstall.InstallBinaries(config); err != nil {
		log.Fatalf("Installation failed: %v", err)
	}
	log.Println("Binaries installed successfully.")
}
```

### CLI Usage

After installing or building the `binaryinstall` CLI, you can run it directly from your terminal. The CLI accepts multiple `-upload` flags to specify one or more tar.gz archives. The binary name is derived from each archive’s filename (e.g. `llmfs_Darwin_arm64.tar.gz` will install a binary named `llmfs`).

For example:

```bash
./binaryinstall \
  -remote ec2-12-34-56-78.compute-1.amazonaws.com \
  -sshuser ec2-user \
  -sshkey /path/to/ssh-key.pem \
  -upload /home/ec2-user/llmfs_Darwin_arm64.tar.gz \
  -upload /home/ec2-user/llmfs_Linux_x86_64.tar.gz \
  -dest /usr/local/bin \
  -backup /home/ec2-user/bin.old \
  -owner root \
  -perm 0755 \
  -verbose
```

This command will:
- Connect to the remote host using the provided SSH credentials.
- Process each specified `-upload` archive.
- Automatically derive the binary name from each archive's filename.
- Install the binary into `/usr/local/bin`, backing up any existing binary into `/home/ec2-user/bin.old`.
- Set the ownership to `root` and permissions to `0755`.
- Output detailed information about each command if the `-verbose` flag is enabled.

## Test

```bash
make test
```

## Release

```bash
make release
```