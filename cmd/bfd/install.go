package main

// "go install" succeeding is not the same as bfd being runnable. It writes to
// GOBIN — or GOPATH/bin — and neither is on PATH by default on most Linux
// installs, so the tool reports success and the very next command a user
// types is "command not found". A second copy earlier on PATH is the quieter
// version of the same problem: the update lands, and the binary that actually
// runs is still the old one.
//
// So an update proves its own result: it runs what it just installed, prints
// the version that answered, and says plainly when PATH will not find it.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type installReportInput struct {
	GoBinary string
}

func installReport(input installReportInput) {
	directory := installDirectoryFind(installDirectoryInput(input))
	if directory == "" {
		fmt.Println(`updated — "bfd version" shows the result`)
		return
	}
	installed := filepath.Join(directory, installBinaryName())

	if output, err := exec.Command(installed, "version").Output(); err == nil {
		fmt.Print(string(output))
	} else {
		fmt.Printf("installed to %s\n", installed)
	}

	resolved, err := exec.LookPath(installBinaryName())
	switch {
	case err != nil:
		fmt.Printf("\n%s is not on your PATH, so this install is not yet runnable by name.\nAdd it:\n\n    export PATH=\"%s:$PATH\"\n\n", directory, directory)
	case !installSamePath(resolved, installed):
		fmt.Printf("\nwarning: %q comes first on your PATH and shadows the copy just installed\nat %s. Remove it, or put %s ahead of it.\n\n", resolved, installed, directory)
	}
}

type installDirectoryInput struct {
	GoBinary string
}

// installDirectoryFind returns where "go install" puts binaries: GOBIN when
// set, otherwise GOPATH/bin.
func installDirectoryFind(input installDirectoryInput) string {
	if binDirectory := installGoEnv(installGoEnvInput{GoBinary: input.GoBinary, Name: "GOBIN"}); binDirectory != "" {
		return binDirectory
	}
	if pathDirectory := installGoEnv(installGoEnvInput{GoBinary: input.GoBinary, Name: "GOPATH"}); pathDirectory != "" {
		return filepath.Join(pathDirectory, "bin")
	}
	return ""
}

type installGoEnvInput struct {
	GoBinary string
	Name     string
}

func installGoEnv(input installGoEnvInput) string {
	output, err := exec.Command(input.GoBinary, "env", input.Name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func installBinaryName() string {
	if runtime.GOOS == "windows" {
		return "bfd.exe"
	}
	return "bfd"
}

// installSamePath compares two paths after resolving symlinks, so a GOBIN
// reached through a link is not reported as a shadow of itself.
func installSamePath(left string, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
