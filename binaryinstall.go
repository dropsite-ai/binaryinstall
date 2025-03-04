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

// BinaryUpload holds info about a single tar.gz upload to install.
type BinaryUpload struct {
	Path           string // path to the tar.gz on remote
	DestinationDir string // install destination (e.g. /usr/local/bin)
	Owner          string // e.g. "root"
	Permission     string // e.g. "0755"
	BindLowPorts   bool   // whether to call setcap for low-numbered port binding
}

// BinaryInstallConfig holds all configuration options needed to install one or more binaries remotely.
type BinaryInstallConfig struct {
	// Remote host connection info.
	RemoteHost string // e.g., "ec2-xx-xx-xx-xx.compute-1.amazonaws.com"
	SSHUser    string // e.g., "ec2-user"
	SSHKeyPath string // e.g., "/path/to/my-key.pem"

	// Uploads is the new structured slice that replaces the old UploadPaths.
	Uploads []BinaryUpload

	// Where to store existing binaries if we back them up.
	BackupDir string

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
    sudo mv "{{.DestinationDir}}/{{.BinaryName}}" "{{.BackupDir}}"/
fi

# 6) Copy the new binary to destination
sudo cp "{{.TempDir}}/{{.BinaryName}}" "{{.DestinationDir}}"

# 7) Set ownership
sudo chown {{.Owner}}:{{.Owner}} "{{.DestinationDir}}/{{.BinaryName}}"

# 8) Set permissions
sudo chmod {{.Permission}} "{{.DestinationDir}}/{{.BinaryName}}"

# 9) Remove the temporary directory
rm -rf "{{.TempDir}}"

{{ if .BindLowPorts }}
# 10) Grant capability to bind to low-numbered ports
sudo setcap 'cap_net_bind_service=+ep' "{{.DestinationDir}}/{{.BinaryName}}"
{{ end }}
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
	BindLowPorts   bool
}

// InstallBinaries processes each tar.gz file in parallel, installing its binary with one SSH command.
func InstallBinaries(config BinaryInstallConfig) error {
	if len(config.Uploads) == 0 {
		return fmt.Errorf("no uploads provided")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(config.Uploads))

	for _, upload := range config.Uploads {
		upload := upload // capture within loop
		wg.Add(1)
		go func() {
			defer wg.Done()
			if config.Verbose {
				log.Printf("Processing upload: %s", upload.Path)
			}
			if err := processUploadSingleCommand(config, upload); err != nil {
				errChan <- fmt.Errorf("failed to process upload '%s': %w", upload.Path, err)
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
func processUploadSingleCommand(config BinaryInstallConfig, upload BinaryUpload) error {
	// Derive the binary name from the archive file. Example:
	// "llmfs_Darwin_arm64.tar.gz" => "llmfs"
	base := filepath.Base(upload.Path)
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
		UploadPath:     upload.Path,
		BinaryName:     binaryName,
		BackupDir:      config.BackupDir,
		DestinationDir: upload.DestinationDir,
		Owner:          upload.Owner,
		Permission:     upload.Permission,
		BindLowPorts:   upload.BindLowPorts,
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
			log.Printf("# SSH script for %s:\n%s", upload.Path, script)
		}
		return err
	}

	if config.Verbose {
		log.Printf("Successfully processed upload: %s (binary: %s)", upload.Path, binaryName)
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
