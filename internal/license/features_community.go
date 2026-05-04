//go:build !pro

package license

// HasFeature is a community stub — no features are available.
func (l *License) HasFeature(_ string) bool {
	return false
}
