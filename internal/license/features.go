//go:build pro

package license

// HasFeature checks whether the active license enables the given feature.
func (l *License) HasFeature(feature string) bool {
	for _, f := range l.Features {
		if f == feature {
			return true
		}
	}
	return false
}
