package license

// Verifier is the interface for license verification.
// Community stub always returns StatusNotFound.
// Pro implementation performs Ed25519 JWT verification.
type Verifier interface {
	// Verify loads and verifies the license file from the configured paths.
	// Returns the parsed license and its status.
	Verify() (*License, Status, error)

	// HasFeature checks whether the active license enables the given feature.
	HasFeature(feature string) bool
}
