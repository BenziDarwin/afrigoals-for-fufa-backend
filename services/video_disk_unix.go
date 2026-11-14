//go:build !windows

// services/video_disk_unix.go
package services

import "syscall"

// availableDiskBytes returns the bytes available to unprivileged writers on the
// filesystem that holds dir.
func availableDiskBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
