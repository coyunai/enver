//go:build !windows

package platform

import "syscall"

// DiskFree returns the bytes available to the current user on the volume.
func DiskFree(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
