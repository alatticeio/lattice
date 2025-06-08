package internal

type EngineManager interface {
	// Start the engine
	Start() error

	// Stop the engine
	Stop() error

	// GetWgConfiger  // Get the WireGuard configuration manager
	GetWgConfiger() ConfigureManager

	GetRelayer() Relay
}
