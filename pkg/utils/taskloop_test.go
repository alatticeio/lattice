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
	"fmt"
	"testing"
	"time"
)

func TestTaskLoop(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		// Create a task loop with queue size 50
		taskLoop := NewTaskLoop(50)

		// Create a context
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Start the loop
		taskLoop.Start(ctx)

		// Continuously add tasks
		for i := 0; i < 100; i++ {
			taskID := i // capture loop variable

			// Add task to the queue
			err := taskLoop.AddTask(ctx, func(ctx context.Context) error {
				fmt.Printf("Executing task %d\n", taskID)
				time.Sleep(100 * time.Millisecond)
				return nil
			})

			if err != nil {
				fmt.Printf("Failed to add task %d: %v\n", i, err)
			}

			// Simulate interval between task arrivals
			time.Sleep(50 * time.Millisecond)
		}

		// Check queue status
		fmt.Printf("Tasks currently queued: %d\n", taskLoop.QueuedTasksCount())

		// Stop the loop
		taskLoop.Stop()

	})
}
