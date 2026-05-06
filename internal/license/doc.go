// Package license provides license verification for Lattice Pro edition.
//
// License files use Ed25519-signed JWT format.
// Community builds always report "not found"; Pro builds perform full verification.
package license

// Feature constants for use with Verifier.HasFeature and requireFeature middleware.
// These must match the strings embedded in the license JWT's "features" claim.
const (
	FeatureDashboard  = "dashboard"
	FeatureMonitoring = "monitoring"
	FeatureAudit      = "audit"
	FeatureOIDC       = "oidc"
	FeatureTURN       = "turn"
	FeatureTelemetry  = "telemetry"
)
