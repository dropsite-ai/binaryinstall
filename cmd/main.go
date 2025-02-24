package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dropsite-ai/binaryinstall" // Adjust the import path as needed.
)

// uploadList collects multiple -upload flags.
type uploadList []string

func (u *uploadList) String() string {
	return strings.Join(*u, ",")
}

func (u *uploadList) Set(value string) error {
	*u = append(*u, value)
	return nil
}

func main() {
	var remoteHost, sshUser, sshKeyPath, destDir, backupDir, owner, permission string
	var verbose bool
	var uploads uploadList

	flag.StringVar(&remoteHost, "remote", "", "Remote host address (required)")
	flag.StringVar(&sshUser, "sshuser", "ec2-user", "SSH user for remote host (default: ec2-user)")
	flag.StringVar(&sshKeyPath, "sshkey", "", "Path to SSH key (required)")
	flag.Var(&uploads, "upload", "Path to an uploaded tar.gz file on remote. Specify multiple times for multiple files (required at least once)")
	flag.StringVar(&destDir, "dest", "/usr/local/bin", "Destination directory on remote for the binary (default: /usr/local/bin)")
	flag.StringVar(&backupDir, "backup", "/home/ec2-user/bin.old", "Backup directory on remote for existing binary (default: /home/ec2-user/bin.old)")
	flag.StringVar(&owner, "owner", "root", "Owner for the installed binary (default: root)")
	flag.StringVar(&permission, "perm", "0755", "Permissions for the installed binary (default: 0755)")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")

	flag.Parse()

	if remoteHost == "" || sshKeyPath == "" || len(uploads) == 0 {
		fmt.Println("Error: -remote, -sshkey, and at least one -upload flag are required.")
		flag.Usage()
		os.Exit(1)
	}

	config := binaryinstall.BinaryInstallConfig{
		RemoteHost:     remoteHost,
		SSHUser:        sshUser,
		SSHKeyPath:     sshKeyPath,
		UploadPaths:    uploads,
		DestinationDir: destDir,
		BackupDir:      backupDir,
		Owner:          owner,
		Permission:     permission,
		Verbose:        verbose,
	}

	if config.Verbose {
		log.Printf("Starting installation on %s", remoteHost)
	}
	if err := binaryinstall.InstallBinaries(config); err != nil {
		log.Fatalf("Installation failed: %v", err)
	}
	if config.Verbose {
		fmt.Println("Binaries installed successfully.")
	}
}
