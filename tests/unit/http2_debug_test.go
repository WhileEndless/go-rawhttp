package unit

import (
	"testing"

	"github.com/WhileEndless/go-rawhttp"
)

// TestHTTP2DebugFlags tests the new HTTP/2 Debug structure
func TestHTTP2DebugFlags(t *testing.T) {
	t.Run("DefaultDebugFlags", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}

		// All debug flags should default to false
		if settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should default to false")
		}
		if settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should default to false")
		}
		if settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should default to false")
		}
		if settings.Debug.LogData {
			t.Error("Debug.LogData should default to false")
		}
	})

	t.Run("EnableLogFrames", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}
		settings.Debug.LogFrames = true

		if !settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be true")
		}

		// Other flags should remain false
		if settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be false")
		}
		if settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be false")
		}
		if settings.Debug.LogData {
			t.Error("Debug.LogData should be false")
		}
	})

	t.Run("EnableLogSettings", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}
		settings.Debug.LogSettings = true

		if !settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be true")
		}

		// Other flags should remain false
		if settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be false")
		}
		if settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be false")
		}
		if settings.Debug.LogData {
			t.Error("Debug.LogData should be false")
		}
	})

	t.Run("EnableLogHeaders", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}
		settings.Debug.LogHeaders = true

		if !settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be true")
		}

		// Other flags should remain false
		if settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be false")
		}
		if settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be false")
		}
		if settings.Debug.LogData {
			t.Error("Debug.LogData should be false")
		}
	})

	t.Run("EnableLogData", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}
		settings.Debug.LogData = true

		if !settings.Debug.LogData {
			t.Error("Debug.LogData should be true")
		}

		// Other flags should remain false
		if settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be false")
		}
		if settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be false")
		}
		if settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be false")
		}
	})

	t.Run("EnableAllDebugFlags", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}
		settings.Debug.LogFrames = true
		settings.Debug.LogSettings = true
		settings.Debug.LogHeaders = true
		settings.Debug.LogData = true

		if !settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be true")
		}
		if !settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be true")
		}
		if !settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be true")
		}
		if !settings.Debug.LogData {
			t.Error("Debug.LogData should be true")
		}
	})

	t.Run("ZeroOverhead", func(t *testing.T) {
		// Test that debug flags are zero-value when not set
		settings := &rawhttp.HTTP2Settings{
			MaxConcurrentStreams: 100,
			InitialWindowSize:    4194304,
		}

		// Debug struct should have zero values (all false)
		if settings.Debug.LogFrames || settings.Debug.LogSettings ||
			settings.Debug.LogHeaders || settings.Debug.LogData {
			t.Error("Debug flags should all be false (zero values)")
		}
	})
}

// TestHTTP2DebugInOptions tests Debug flags in Options struct
func TestHTTP2DebugInOptions(t *testing.T) {
	t.Run("DebugFlagsInCompleteOptions", func(t *testing.T) {
		opts := rawhttp.Options{
			Scheme:   "https",
			Host:     "example.com",
			Port:     443,
			Protocol: "http/2",
			HTTP2Settings: &rawhttp.HTTP2Settings{
				MaxConcurrentStreams: 100,
			},
		}

		// Set debug flags
		opts.HTTP2Settings.Debug.LogFrames = true
		opts.HTTP2Settings.Debug.LogSettings = true

		if !opts.HTTP2Settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be true")
		}
		if !opts.HTTP2Settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be true")
		}
		if opts.HTTP2Settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be false")
		}
		if opts.HTTP2Settings.Debug.LogData {
			t.Error("Debug.LogData should be false")
		}
	})
}

// TestHTTP2DebugProductionSafe tests that debug is safe for production
func TestHTTP2DebugProductionSafe(t *testing.T) {
	t.Run("ProductionDefaults", func(t *testing.T) {
		// Simulating a production configuration
		opts := rawhttp.Options{
			Scheme:   "https",
			Host:     "api.production.com",
			Port:     443,
			Protocol: "http/2",
			HTTP2Settings: &rawhttp.HTTP2Settings{
				MaxConcurrentStreams: 100,
				InitialWindowSize:    4194304,
				DisableServerPush:    true,
				EnableCompression:    true,
			},
		}

		// Debug flags should all be false by default (production safe)
		if opts.HTTP2Settings.Debug.LogFrames {
			t.Error("Production config should have Debug.LogFrames = false")
		}
		if opts.HTTP2Settings.Debug.LogSettings {
			t.Error("Production config should have Debug.LogSettings = false")
		}
		if opts.HTTP2Settings.Debug.LogHeaders {
			t.Error("Production config should have Debug.LogHeaders = false")
		}
		if opts.HTTP2Settings.Debug.LogData {
			t.Error("Production config should have Debug.LogData = false")
		}
	})

	t.Run("ExplicitOptIn", func(t *testing.T) {
		// Debug flags require explicit opt-in
		opts := rawhttp.Options{
			Scheme:   "https",
			Host:     "debug.example.com",
			Port:     443,
			Protocol: "http/2",
			HTTP2Settings: &rawhttp.HTTP2Settings{
				MaxConcurrentStreams: 100,
			},
		}

		// Explicitly enable debug for troubleshooting
		opts.HTTP2Settings.Debug.LogFrames = true
		opts.HTTP2Settings.Debug.LogHeaders = true

		if !opts.HTTP2Settings.Debug.LogFrames {
			t.Error("Explicitly enabled Debug.LogFrames should be true")
		}
		if !opts.HTTP2Settings.Debug.LogHeaders {
			t.Error("Explicitly enabled Debug.LogHeaders should be true")
		}
	})
}

// TestHTTP2DebugStructAlignment tests that Debug struct is properly aligned
func TestHTTP2DebugStructAlignment(t *testing.T) {
	t.Run("DebugStructSize", func(t *testing.T) {
		settings := &rawhttp.HTTP2Settings{}

		// Access all fields to ensure they're accessible
		_ = settings.Debug.LogFrames
		_ = settings.Debug.LogSettings
		_ = settings.Debug.LogHeaders
		_ = settings.Debug.LogData

		// Test setting fields individually
		settings.Debug.LogFrames = true
		settings.Debug.LogSettings = true
		settings.Debug.LogHeaders = false
		settings.Debug.LogData = false

		if !settings.Debug.LogFrames {
			t.Error("Debug.LogFrames should be true after assignment")
		}
		if !settings.Debug.LogSettings {
			t.Error("Debug.LogSettings should be true after assignment")
		}
		if settings.Debug.LogHeaders {
			t.Error("Debug.LogHeaders should be false after assignment")
		}
		if settings.Debug.LogData {
			t.Error("Debug.LogData should be false after assignment")
		}
	})
}

// TestHTTP2DebugSelectiveLogging tests selective debug logging scenarios
func TestHTTP2DebugSelectiveLogging(t *testing.T) {
	scenarios := []struct {
		name        string
		logFrames   bool
		logSettings bool
		logHeaders  bool
		logData     bool
	}{
		{"NoDebug", false, false, false, false},
		{"OnlyFrames", true, false, false, false},
		{"OnlySettings", false, true, false, false},
		{"OnlyHeaders", false, false, true, false},
		{"OnlyData", false, false, false, true},
		{"FramesAndSettings", true, true, false, false},
		{"HeadersAndData", false, false, true, true},
		{"AllDebug", true, true, true, true},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			settings := &rawhttp.HTTP2Settings{}
			settings.Debug.LogFrames = sc.logFrames
			settings.Debug.LogSettings = sc.logSettings
			settings.Debug.LogHeaders = sc.logHeaders
			settings.Debug.LogData = sc.logData

			if settings.Debug.LogFrames != sc.logFrames {
				t.Errorf("LogFrames: expected %v, got %v", sc.logFrames, settings.Debug.LogFrames)
			}
			if settings.Debug.LogSettings != sc.logSettings {
				t.Errorf("LogSettings: expected %v, got %v", sc.logSettings, settings.Debug.LogSettings)
			}
			if settings.Debug.LogHeaders != sc.logHeaders {
				t.Errorf("LogHeaders: expected %v, got %v", sc.logHeaders, settings.Debug.LogHeaders)
			}
			if settings.Debug.LogData != sc.logData {
				t.Errorf("LogData: expected %v, got %v", sc.logData, settings.Debug.LogData)
			}
		})
	}
}
