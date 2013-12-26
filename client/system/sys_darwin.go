package system

import (
	"os"
	"crypto/rand"
	"os/exec"
	"strings"
	"path/filepath"
	"sync"
	"fmt"
)

// OS X doesn't appear to have a tmpfs mounted by default. However, it does appear to use encrypted swap by default, which is good. (Try running `sysctl vm.swapusage` to check.) So this function mounts a tmpfs on ~/.pondtmp and unmounts it on exit (assuming that we don't crash). It would be nice if we could open a file descriptor to the directory and then lazy unmount it. However, Darwin doesn't appear to have lazy unmount, nor openat().

var (
	// safeTempDir contains the name of a directory that is a RAM disk mount once setupSafeTempDir has been run, unless safeTempDirErr is non-nil.
	safeTempDir     string
	// safeTempDirErr contains any errors arising from trying to setup a RAM disk by setupSafeTempDir.
	safeTempDirErr  error
	// safeTempDevice, if not empty, contains the device name of the RAM disk created by setupSafeTempDir.
	safeTempDevice string
	// safeTempMounted is true if setupSafeTempDir mounted safeTempDevice on /Volumes/$safeTempVolumeName.
	safeTempMounted bool
    // safeTempDirOnce protects setupSafeTempDir from running multiple times.
	safeTempDirOnce sync.Once
	// safeTempVolumName contains the name of the RAM disk volume that we'll create. This turns into a directory name in /Volumes and also appears in the Disk Utility GUI when Pond is running.
	safeTempVolumeName string
)

func runCommandWithIO(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupSafeTempDir() {
	var randBytes [6]byte
	rand.Reader.Read(randBytes[:])
    safeTempVolumeName = fmt.Sprintf("Pond RAM disk (%x)", randBytes)

    hdiUtilOutput, err := exec.Command("hdiutil", "attach", "-nomount", "ram://2048").CombinedOutput()
    if err != nil {
    	safeTempDirErr = err
    	return
    }
    device := strings.TrimSpace(string(hdiUtilOutput))
    safeTempDevice = device
    if err := exec.Command("diskutil", "erasevolume", "HFS+", safeTempVolumeName, device).Run(); err != nil {
    	safeTempDirErr = err
    	return
    }

    safeTempMounted = true
    safeTempDir = "/Volumes/" + safeTempVolumeName

    readMe, err := os.OpenFile(filepath.Join(safeTempDir, "README"), os.O_CREATE | os.O_WRONLY | os.O_TRUNC, 0644)
    if err != nil {
    	safeTempDirErr = err
    	return
    }
    defer readMe.Close()

    fmt.Fprintf(readMe, `Pond Safe Temp Directory

This directory contains a RAM filesystem, created by Pond, so that temporary files
can be safely used. Unless Pond is still running, it has somehow failed to cleanup
after itself! Sorry! You can run the following commands to clean it up:

$ cd ~
$ umount "/Volumes/%s"
$ hdiutil detach %s
`, safeTempVolumeName, device)
}

func SafeTempDir() (string, error) {
    safeTempDirOnce.Do(setupSafeTempDir)
	if safeTempDirErr != nil {
		return "", safeTempDirErr
	}
	return safeTempDir, nil
}

func Shutdown() {
	if safeTempMounted {
		runCommandWithIO("umount", "/Volumes/" + safeTempVolumeName)
		safeTempMounted = false
	}
	if len(safeTempDevice) > 0 {
		runCommandWithIO("hdiutil", "detach", safeTempDevice)
		safeTempDevice = ""
	}
}