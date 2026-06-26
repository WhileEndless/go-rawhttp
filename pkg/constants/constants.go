// Package constants defines magic numbers and default values used throughout go-rawhttp
package constants

import "time"

// Connection timeouts and limits
const (
	DefaultIdleTimeout     = 90 * time.Second
	DefaultConnTimeout     = 10 * time.Second
	DefaultReadTimeout     = 30 * time.Second
	DefaultPingInterval    = 15 * time.Second
	MaxConnectionIdleTime  = 5 * time.Minute
	HealthCheckInterval    = 30 * time.Second
	CleanupInterval        = 30 * time.Second
)

// HTTP/2 limits
const (
	MaxTotalStreams       = 10000
	SettingsAckTimeout    = 10 * time.Second
	DefaultHpackTableSize = 4096
)

// HTTP limits
const (
	MaxContentLength = 1024 * 1024 * 1024 * 1024 // 1TB
)

// Buffer limits
const (
	DefaultBodyMemLimit = 4 * 1024 * 1024 // 4MB
	MaxRawBufferSize    = 100 * 1024 * 1024 // 100MB cap for raw buffer
)
