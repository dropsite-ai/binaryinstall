package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dropsite-ai/binaryinstall"
)

// uploadSpec is a custom type for parsing key=value pairs passed to -upload.
type uploadSpec struct {
	binaryinstall.BinaryUpload
}

func (u *uploadSpec) String() string {
	// Return a short identifier for debugging (not strictly needed).
	return fmt.Sprintf("path=%s,dest=%s,owner=%s,perm=%s,bindlowports=%t",
		u.Path, u.DestinationDir, u.Owner, u.Permission, u.BindLowPorts)
}

// Set parses a string like "path=/x.tar.gz,dest=/usr/local/bin,owner=root,perm=0755,bindlowports=true"
func (u *uploadSpec) Set(value string) error {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid upload argument: %q", part)
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])

		switch key {
		case "path":
			u.Path = val
		case "dest":
			u.DestinationDir = val
		case "owner":
			u.Owner = val
		case "perm":
			u.Permission = val
		case "bindlowports":
			lower := strings.ToLower(val)
			u.BindLowPorts = (lower == "true" || lower == "1" || lower == "yes")
		default:
			return fmt.Errorf("unknown field %q in upload spec", key)
		}
	}

	// Provide some defaults if desired:
	if u.DestinationDir == "" {
		u.DestinationDir = "/usr/local/bin"
	}
	if u.Owner == "" {
		u.Owner = "root"
	}
	if u.Permission == "" {
		u.Permission = "0755"
	}

	return nil
}

// uploadList is a slice of uploadSpec that implements flag.Value
type uploadList []binaryinstall.BinaryUpload

func (ul *uploadList) String() string {
	var out []string
	for _, u := range *ul {
		out = append(out, fmt.Sprintf("path=%s", u.Path))
	}
	return strings.Join(out, "; ")
}

func (ul *uploadList) Set(value string) error {
	var us uploadSpec
	if err := us.Set(value); err != nil {
		return err
	}
	*ul = append(*ul, us.BinaryUpload)
	return nil
}

func main() {
	var (
		remoteHost string
		sshUser    string
		sshKeyPath string
		backupDir  string
		verbose    bool
		uploads    uploadList
	)

	flag.StringVar(&remoteHost, "remote", "", "Remote host address (required)")
	flag.StringVar(&sshUser, "sshuser", "ec2-user", "SSH user for remote host (default: ec2-user)")
	flag.StringVar(&sshKeyPath, "sshkey", "", "Path to SSH key (required)")
	flag.Var(&uploads, "upload", "Specify an upload in the form \"path=/x.tar.gz,dest=/usr/local/bin,owner=root,perm=0755,bindlowports=true\" (can be repeated)")
	flag.StringVar(&backupDir, "backup", "/home/ec2-user/bin.old", "Backup directory on remote (default: /home/ec2-user/bin.old)")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")

	flag.Parse()

	if remoteHost == "" || sshKeyPath == "" || len(uploads) == 0 {
		fmt.Println("Error: -remote, -sshkey, and at least one -upload flag are required.")
		flag.Usage()
		os.Exit(1)
	}

	config := binaryinstall.BinaryInstallConfig{
		RemoteHost: remoteHost,
		SSHUser:    sshUser,
		SSHKeyPath: sshKeyPath,
		Uploads:    uploads,
		BackupDir:  backupDir,
		Verbose:    verbose,
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
