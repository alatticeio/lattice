// Copyright 2025 wireflowio.com, Inc.
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

package dns

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Record struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  int    `json:"ttl"`
	Data string `json:"data"`
}

// DnsClient root dns using coredns, client will add/remove a record to coredns.
type DnsClient interface {
	AddRecord(record Record) error
	RemoveRecord(record Record) error
}

type client struct {
	httpClient http.Client
}

func NewDnsClient() DnsClient {
	return &client{
		httpClient: http.Client{},
	}
}

func (c *client) AddRecord(record Record) error {
	var (
		err error
		bs  []byte
	)
	bs, err = json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = c.httpClient.Post(
		"http://linkany.io:9001/api/v1/zones/example.com",
		"Content-Type: application/json",
		bytes.NewReader(bs),
	)
	return err
}

func (c *client) RemoveRecord(record Record) error {
	return nil
}
