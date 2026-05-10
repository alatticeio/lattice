// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package license

// NewPlaygroundVerifier returns a Verifier that grants all Pro features
// without requiring a license file. Intended for playground/demo deployments only.
func NewPlaygroundVerifier() Verifier {
	return &playgroundVerifier{}
}

type playgroundVerifier struct{}

func (v *playgroundVerifier) Verify() (*License, Status, error) {
	return &License{Type: "playground"}, StatusValid, nil
}

func (v *playgroundVerifier) HasFeature(_ string) bool { return true }
