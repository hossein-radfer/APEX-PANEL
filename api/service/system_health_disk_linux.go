//go:build linux

package service

import "syscall"

// readDisk reports free/used space for the filesystem backing
// DataDirPath via syscall.Statfs -- see system_health.go's own doc
// comment for why this is split into a per-OS file.
func (s *SystemHealthService) readDisk() (*DiskHealth, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.dataDirPath, &stat); err != nil {
		return nil, err
	}

	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return &DiskHealth{
		TotalBytes: total,
		FreeBytes:  free,
		UsedBytes:  total - free,
	}, nil
}
