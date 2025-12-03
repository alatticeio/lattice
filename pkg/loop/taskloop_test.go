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

package loop

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTaskLoop(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		// 创建一个队列大小为 50 的任务循环
		taskLoop := NewTaskLoop(50)

		// 创建上下文
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 启动循环
		taskLoop.Start(ctx)

		// 不断添加任务
		for i := 0; i < 100; i++ {
			taskID := i // 捕获变量

			// 添加任务到队列
			err := taskLoop.AddTask(ctx, func(ctx context.Context) error {
				fmt.Printf("执行任务 %d\n", taskID)
				time.Sleep(100 * time.Millisecond)
				return nil
			})

			if err != nil {
				fmt.Printf("添加任务 %d 失败: %v\n", i, err)
			}

			// 模拟任务到达的间隔
			time.Sleep(50 * time.Millisecond)
		}

		// 检查队列状态
		fmt.Printf("当前队列中等待的任务: %d\n", taskLoop.QueuedTasksCount())

		// 停止循环
		taskLoop.Stop()

	})
}
