package config

// Config holds EcoScale runtime configuration.
type Config struct {
	// Carbon API
	CarbonAPI    string  // mock | carbonintensity | electricitymaps
	CarbonAPIKey string  // ElectricityMaps API key (required when CarbonAPI=electricitymaps)
	CarbonThreshold float64

	// Safety
	DryRun         bool    // If true, only recommend; never execute actions
	EvictionCapPct float64 // Max % of flexible pods to evict per cycle (0-100)
	ProtectedLabel string  // Pods with this label are never evicted

	// Execution
	EnableExecution bool // If true and not DryRun, execute drain/evict actions
}

// DefaultConfig returns production-safe defaults.
func DefaultConfig() Config {
	return Config{
		CarbonAPI:       "mock",
		CarbonAPIKey:    "",
		CarbonThreshold: 350,

		DryRun:         true, // Safe default: recommendations only
		EvictionCapPct: 10,
		ProtectedLabel: "ecoscale/protected",

		EnableExecution: false,
	}
}
