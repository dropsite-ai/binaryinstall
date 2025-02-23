package binaryinstall

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
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
	Owner      string // e.g., "root" or "systemgate"
	Permission string // e.g., "0755"

	// Verbose mode: if true, prints out each command and its status.
	Verbose bool
}

// InstallBinaries processes each uploaded tar.gz file and installs the binary.
func InstallBinaries(config BinaryInstallConfig) error {
	if len(config.UploadPaths) == 0 {
		return fmt.Errorf("no upload paths provided")
	}

	for _, uploadPath := range config.UploadPaths {
		if config.Verbose {
			log.Printf("Processing upload: %s", uploadPath)
		}
		if err := processUpload(config, uploadPath); err != nil {
			return fmt.Errorf("failed to process upload '%s': %w", uploadPath, err)
		}
	}
	return nil
}

// processUpload processes a single upload file.
func processUpload(config BinaryInstallConfig, uploadPath string) error {
	// Derive binary name from the archive file.
	// Example: "llmfs_Darwin_arm64.tar.gz" => "llmfs"
	base := filepath.Base(uploadPath)
	nameWithoutExt := strings.TrimSuffix(base, ".tar.gz")
	parts := strings.Split(nameWithoutExt, "_")
	if len(parts) == 0 {
		return fmt.Errorf("unable to derive binary name from %s", base)
	}
	binaryName := parts[0]

	// Create a temporary extraction directory on the remote host.
	tempDir := fmt.Sprintf("/tmp/install-%d", time.Now().UnixNano())
	mkdirCmd := fmt.Sprintf("mkdir -p %s", tempDir)
	if _, err := executeSSHCommand(config, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Unpack the tar.gz file into the temporary directory.
	untarCmd := fmt.Sprintf("tar -xzf %s -C %s", uploadPath, tempDir)
	if _, err := executeSSHCommand(config, untarCmd); err != nil {
		return fmt.Errorf("failed to extract tar.gz file: %w", err)
	}

	// Determine the full path to the new binary after extraction.
	newBinaryPath := filepath.Join(tempDir, binaryName)
	// Ensure the expected binary exists.
	checkCmd := fmt.Sprintf("test -f %s", newBinaryPath)
	if _, err := executeSSHCommand(config, checkCmd); err != nil {
		return fmt.Errorf("new binary %s not found after extraction: %w", newBinaryPath, err)
	}

	// Ensure the backup directory exists.
	mkdirBackupCmd := fmt.Sprintf("mkdir -p %s", config.BackupDir)
	if _, err := executeSSHCommand(config, mkdirBackupCmd); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Define the destination path for the binary.
	destBinaryPath := filepath.Join(config.DestinationDir, binaryName)

	// If a binary already exists at the destination, back it up.
	backupCmd := fmt.Sprintf("if [ -f %s ]; then mv %s %s/; fi", destBinaryPath, destBinaryPath, config.BackupDir)
	if _, err := executeSSHCommand(config, backupCmd); err != nil {
		return fmt.Errorf("failed to back up existing binary: %w", err)
	}

	// Copy the new binary to the destination directory.
	copyCmd := fmt.Sprintf("cp %s %s", newBinaryPath, config.DestinationDir)
	if _, err := executeSSHCommand(config, copyCmd); err != nil {
		return fmt.Errorf("failed to copy new binary to destination: %w", err)
	}

	// Set the owner (using sudo in case elevated privileges are needed).
	chownCmd := fmt.Sprintf("sudo chown %s:%s %s", config.Owner, config.Owner, destBinaryPath)
	if _, err := executeSSHCommand(config, chownCmd); err != nil {
		return fmt.Errorf("failed to change ownership: %w", err)
	}

	// Set the permissions.
	chmodCmd := fmt.Sprintf("sudo chmod %s %s", config.Permission, destBinaryPath)
	if _, err := executeSSHCommand(config, chmodCmd); err != nil {
		return fmt.Errorf("failed to change permissions: %w", err)
	}

	// Clean up the temporary extraction directory.
	cleanupCmd := fmt.Sprintf("rm -rf %s", tempDir)
	if _, err := executeSSHCommand(config, cleanupCmd); err != nil {
		return fmt.Errorf("failed to clean up temporary directory: %w", err)
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
			log.Printf("Command failed: %s\nError: %v\nOutput: %s", command, err, output)
		} else {
			log.Printf("Command succeeded: %s\nOutput: %s", command, output)
		}
	}
	if err != nil {
		return output, fmt.Errorf("command '%s' failed: %v; output: %s", command, err, output)
	}
	return output, nil
}
