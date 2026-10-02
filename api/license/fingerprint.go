package license

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// installIDFilename is stored inside AppConfig.DataDirPath -- the same
// isolated, persistent directory the SQLite database and PeerFilesDir
// already live in, which is never touched by a binary/container upgrade
// (see this package's doc comment on the "seamless updates" guarantee).
const installIDFilename = "install-id"

// Fingerprint derives a stable identifier for the machine MWPanel is
// running on, used by license-panel to bind a license key to a specific
// server. It combines a persisted install UUID (see EnsureInstallID)
// with a secondary hardware-derived signal, chosen to remain stable
// across routine OS/network reconfiguration and binary/version upgrades.
// This is not a hardware DRM mechanism; it exists so that copying an
// installation to a second server is visible to license-panel, not to
// resist a determined attacker with source access.
func Fingerprint(dataDirPath string) string {
	var parts []string

	if installID, err := EnsureInstallID(dataDirPath); err == nil && installID != "" {
		parts = append(parts, installID)
	}

	if mac := firstPhysicalMAC(); mac != "" {
		parts = append(parts, mac)
	}

	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// EnsureInstallID reads the persisted install UUID from
// <dataDirPath>/install-id, generating and writing a fresh one on first
// run if the file doesn't exist yet. Exported so callers that need the
// raw install ID directly (not hashed into the combined fingerprint) can
// get it without re-deriving the file path themselves.
//
// dataDirPath MUST be the same directory across every restart and every
// version upgrade for the fingerprint to stay stable -- this is exactly
// AppConfig.DataDirPath, which this codebase already treats as the one
// persistent, upgrade-safe location (the SQLite database and
// PeerFilesDir live here too, and nothing in the build/deploy process
// ever recreates or wipes it).
func EnsureInstallID(dataDirPath string) (string, error) {
	path := filepath.Join(dataDirPath, installIDFilename)

	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		// A malformed/corrupt value is treated the same as "file doesn't
		// exist" below and regenerated, same as the empty-file case.
		if _, parseErr := uuid.Parse(id); parseErr == nil {
			return id, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	id := uuid.New().String()
	// 0o600: this file is this install's half of its license binding --
	// no reason for any other local user/process to be able to read it.
	if err := os.WriteFile(path, []byte(id), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

// physicalInterfacePrefixes lists common Linux virtual/tunnel/container
// interface name prefixes to skip when looking for "the first physical
// NIC" -- eth0/en*/wlan0 are physical, everything below is not. This list
// is deliberately conservative (skip-list, not an allow-list of exact
// physical names) since physical interface naming varies across distros
// and hardware (eth0, enp3s0, ens18, wlan0, ...).
var physicalInterfaceSkipPrefixes = []string{
	"lo",       // loopback
	"docker",   // Docker bridge
	"br-",      // generic Linux bridge (including Docker's default bridge naming)
	"veth",     // Docker/container virtual ethernet pair
	"virbr",    // libvirt bridge
	"vnet",     // libvirt tap devices
	"tun",      // OpenVPN/WireGuard-style tunnel devices
	"tap",      // tap devices
	"wg",       // WireGuard interfaces
	"vxlan",    // virtual overlay networks
	"dummy",    // dummy interfaces
	"bond",     // bonded/team virtual interface (the underlying physical slaves are what should match instead)
	"team",
}

// firstPhysicalMAC returns the MAC address of the first network interface
// that looks like real hardware rather than a virtual/tunnel/container
// device, in the order the OS reports them. Interfaces are typically
// enumerated in a stable, kernel-assigned order that does not change
// across reboots for a given physical machine, which is what makes "the
// first one" a meaningful, repeatable signal rather than an arbitrary
// pick.
func firstPhysicalMAC() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.HardwareAddr.String() == "" {
			continue
		}

		name := strings.ToLower(iface.Name)
		skip := false
		for _, prefix := range physicalInterfaceSkipPrefixes {
			if strings.HasPrefix(name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		return iface.HardwareAddr.String()
	}

	return ""
}
