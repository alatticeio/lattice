//go:build !pro

package license

import (
	"errors"
)

// NewVerifier creates a Community-edition license verifier stub.
// Community editions always report "not found" and have no features.
func NewVerifier() Verifier {
	return &communityVerifier{}
}

type communityVerifier struct{}

func (v *communityVerifier) Verify() (*License, Status, error) {
	return nil, StatusNotFound, errors.New("licensing is a Pro feature")
}

func (v *communityVerifier) HasFeature(_ string) bool {
	return false
}
