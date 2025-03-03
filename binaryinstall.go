package binaryinstall

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

// BinaryInstallConfig holds all configuration options needed to install one or more binaries remotely.
type BinaryInstallConfig struct {
	// Remote host connection info.
	RemoteHost string // e.g., "ec2-xx-xx-xx-xx.compute-1.amazonaws.com"
	SSHUser    string // e.g., "ec2-user"
	SSHKeyPath string // e.g., "/path/to/my-key.pem"

	// File locations.
	UploadPaths    []string // List of full paths to the uploaded tar.gz files on the remote host.
	DestinationDir string   // Destination directory for the binary (e.g., "/usr/local/bin")
	BackupDir      string   // Backup directory for any existing binary (e.g., "/home/ec2-user/bin.old")

	// Ownership and permissions.
	Owner      string // e.g., "root"
	Permission string // e.g., "0755"

	// Verbose mode: if true, prints out each command and its status.
	Verbose bool
}

// scriptTemplate is a template for the entire one-shot remote script.
// We'll fill in values with the ScriptData struct below.
var scriptTemplate = template.Must(template.New("sshScript").Parse(`
set -e

# 1) Make the temporary directory
mkdir -p {{.TempDir}}

# 2) Extract the tarball
tar -xzf "{{.UploadPath}}" -C "{{.TempDir}}"

# 3) Verify the new binary exists
test -f "{{.TempDir}}/{{.BinaryName}}"

# 4) Ensure backup directory exists
mkdir -p "{{.BackupDir}}"

# 5) Backup existing binary if it exists
if [ -f "{{.DestinationDir}}/{{.BinaryName}}" ]; then
    mv "{{.DestinationDir}}/{{.BinaryName}}" "{{.BackupDir}}"/
fi

# 6) Copy the new binary to destination
cp "{{.TempDir}}/{{.BinaryName}}" "{{.DestinationDir}}"

# 7) Set ownership
sudo chown {{.Owner}}:{{.Owner}} "{{.DestinationDir}}/{{.BinaryName}}"

# 8) Set permissions
sudo chmod {{.Permission}} "{{.DestinationDir}}/{{.BinaryName}}"

# 9) Remove the temporary directory
rm -rf "{{.TempDir}}"
`))

// ScriptData holds data we'll substitute into scriptTemplate.
type ScriptData struct {
	TempDir        string
	UploadPath     string
	BinaryName     string
	BackupDir      string
	DestinationDir string
	Owner          string
	Permission     string
}

// InstallBinaries processes each uploaded tar.gz file in parallel and installs the binary with a single SSH command.
func InstallBinaries(config BinaryInstallConfig) error {
	if len(config.UploadPaths) == 0 {
		return fmt.Errorf("no upload paths provided")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(config.UploadPaths))

	for _, upload := range config.UploadPaths {
		upload := upload // capture within loop
		wg.Add(1)
		go func() {
			defer wg.Done()
			if config.Verbose {
				log.Printf("Processing upload: %s", upload)
			}
			if err := processUploadSingleCommand(config, upload); err != nil {
				errChan <- fmt.Errorf("failed to process upload '%s': %w", upload, err)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}
	return nil
}

// processUploadSingleCommand does every step in one single SSH call
// by rendering scriptTemplate with the appropriate data.
func processUploadSingleCommand(config BinaryInstallConfig, uploadPath string) error {
	// Derive the binary name from the archive file. Example:
	// "llmfs_Darwin_arm64.tar.gz" => "llmfs"
	base := filepath.Base(uploadPath)
	nameWithoutExt := strings.TrimSuffix(base, ".tar.gz")
	parts := strings.Split(nameWithoutExt, "_")
	if len(parts) == 0 {
		return fmt.Errorf("unable to derive binary name from %s", base)
	}
	binaryName := parts[0]

	// Create a unique temp directory name
	tempDir := fmt.Sprintf("/tmp/install-%d", time.Now().UnixNano())

	// Prepare data for the template
	sData := ScriptData{
		TempDir:        tempDir,
		UploadPath:     uploadPath,
		BinaryName:     binaryName,
		BackupDir:      config.BackupDir,
		DestinationDir: config.DestinationDir,
		Owner:          config.Owner,
		Permission:     config.Permission,
	}

	// Render the template
	var scriptBuf bytes.Buffer
	if err := scriptTemplate.Execute(&scriptBuf, sData); err != nil {
		return fmt.Errorf("failed to render SSH script template: %w", err)
	}
	script := scriptBuf.String()

	// Execute that one big script remotely with SSH.
	if _, err := executeSSHCommand(config, script); err != nil {
		if config.Verbose {
			log.Printf("# SSH script for %s:\n%s", uploadPath, script)
		}
		return err
	}

	if config.Verbose {
		log.Printf("Successfully processed upload: %s (binary: %s)", uploadPath, binaryName)
	}
	return nil
}

// executeSSHCommand runs a given command on the remote host using SSH.
// It prints the command and its status if Verbose is enabled.
func executeSSHCommand(config BinaryInstallConfig, command string) (string, error) {
	sshTarget := fmt.Sprintf("%s@%s", config.SSHUser, config.RemoteHost)
	fullCmd := fmt.Sprintf("ssh -i %s %s '%s'", config.SSHKeyPath, sshTarget, command)
	if config.Verbose {
		log.Printf("Running command: %s", fullCmd)
	}

	cmd := exec.Command("ssh", "-i", config.SSHKeyPath, sshTarget, command)
	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)

	if config.Verbose {
		if err != nil {
			log.Printf("Command failed.\nError: %v\nOutput: %s", err, output)
		} else {
			log.Printf("Command succeeded.\nOutput: %s", output)
		}
	}

	if err != nil {
		return output, fmt.Errorf("command failed: %v; output: %s", err, output)
	}
	return output, nil
}
