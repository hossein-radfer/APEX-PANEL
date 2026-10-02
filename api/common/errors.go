package common

import "errors"

var (
	ErrPeerNotShared               = errors.New("peer is not shared")
	ErrUserManagerAccountNotShared = errors.New("user manager account is not shared")
	ErrV2RayPackageNotShared       = errors.New("v2ray package is not shared")
	ErrDNSAccountNotShared         = errors.New("dns account is not shared")
)
