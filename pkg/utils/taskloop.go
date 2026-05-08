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

package utils

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Task func(ctx context.Context) error

// TaskLoop executes tasks sequentially
type TaskLoop struct {
	tasks chan Task
	mu    sync.Mutex

	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewTaskLoop creates a new TaskLoop with an optional queue size
func NewTaskLoop(queueSize int) *TaskLoop {
	if queueSize <= 0 {
		queueSize = 100 // default queue size
	}
	l := &TaskLoop{
		tasks:  make(chan Task, queueSize),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	l.Start(context.Background())
	return l
}

func (l *TaskLoop) AddTask(ctx context.Context, task Task) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.stopCh:
		return context.Canceled
	case l.tasks <- task:
		return nil
	}
}

func (l *TaskLoop) Start(ctx context.Context) {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.running = true
	l.stopCh = make(chan struct{})
	l.doneCh = make(chan struct{})
	l.mu.Unlock()
	go func() {
		defer close(l.doneCh)
		for {
			select {
			case <-l.stopCh:
				// Process remaining tasks
				l.drainTasksOnStop(ctx)
				return
			case <-ctx.Done():
				// Process remaining tasks
				l.drainTasksOnStop(ctx)
				return
			case task, ok := <-l.tasks:
				if !ok {
					return
				}
				// Execute the task, ignoring errors
				_ = task(ctx)
			}
		}
	}()

}

// drainTasksOnStop processes remaining tasks when stopping
func (l *TaskLoop) drainTasksOnStop(ctx context.Context) {
	// Create a 1-second timeout context for processing remaining tasks
	drainCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Attempt to process remaining tasks
	for {
		select {
		case <-drainCtx.Done():
			// Timeout, abandon remaining tasks
			return
		case task, ok := <-l.tasks:
			if !ok {
				return
			}
			// Attempt to execute remaining tasks
			_ = task(ctx)
		default:
			// No more tasks
			return
		}
	}
}

// Stop stops the task loop and waits for all in-flight tasks to complete
func (l *TaskLoop) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.running = false
	close(l.stopCh)
	l.mu.Unlock()

	<-l.doneCh // Wait for processing to complete
}

// TryAddTask attempts to add a task, returning immediately if the queue is full
func (l *TaskLoop) TryAddTask(task Task) error {
	select {
	case <-l.stopCh:
		return context.Canceled
	case l.tasks <- task:
		return nil
	default:
		return errors.New("task queue is full")
	}
}

// IsRunning returns whether the loop is running
func (l *TaskLoop) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// QueuedTasksCount returns the number of tasks currently queued
func (l *TaskLoop) QueuedTasksCount() int {
	return len(l.tasks)
}
