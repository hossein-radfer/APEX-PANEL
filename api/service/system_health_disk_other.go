//go:build !linux

package service

import "fmt"

// readDisk on non-Linux platforms (Windows local dev, etc.) -- this
// panel's own documented deploy target is always Linux/systemd (see
// system_health.go's own doc comment), so this stub only exists to keep
// local development builds compiling on other platforms; it is never
// exercised in production.
func (s *SystemHealthService) readDisk() (*DiskHealth, error) {
	return nil, fmt.Errorf("disk usage reporting is only implemented for linux")
}
