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

package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FileLogger 将审计日志写入 JSONL 文件
type FileLogger struct {
	path string
	mu   sync.Mutex
	file *os.File
}

// NewFileLogger 创建一个新的文件审计日志器
func NewFileLogger(path string) (*FileLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &FileLogger{
		path: path,
		file: f,
	}, nil
}

func (l *FileLogger) Log(entry AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return
	}

	// 设置默认时间戳
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 序列化为 JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// 写入文件（JSONL 格式，每行一条记录）
	data = append(data, '\n')
	_, _ = l.file.Write(data)
}

func (l *FileLogger) LogError(entry AuditEntry, err error) {
	entry.Result = "error"
	entry.ErrorMessage = err.Error()
	l.Log(entry)
}

func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// GetPath 返回日志文件路径
func (l *FileLogger) GetPath() string {
	return l.path
}
