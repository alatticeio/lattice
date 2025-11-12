// Copyright 2025 Wireflow.io, Inc.
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

package config

// User is a linkany user, will be used to login and store token in local
// user use token to fetch config from linkany center
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// NewUser will create a new user
func NewUser(username, password string) *User {
	return &User{
		Username: username,
		Password: password,
	}
}
