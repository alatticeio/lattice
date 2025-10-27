package loop

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Task func(ctx context.Context) error

// TaskLoop 顺序执行任务
type TaskLoop struct {
	tasks chan Task
	mu    sync.Mutex

	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewTaskLoop 创建一个新的 TaskLoop，可以指定队列大小
func NewTaskLoop(queueSize int) *TaskLoop {
	if queueSize <= 0 {
		queueSize = 100 // 默认队列大小
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
				// 处理剩余任务
				l.drainTasksOnStop(ctx)
				return
			case <-ctx.Done():
				// 处理剩余任务
				l.drainTasksOnStop(ctx)
				return
			case task, ok := <-l.tasks:
				if !ok {
					return
				}
				// 执行任务，忽略错误
				_ = task(ctx)
			}
		}
	}()

}

// drainTasksOnStop 在停止时处理剩余的任务
func (l *TaskLoop) drainTasksOnStop(ctx context.Context) {
	// 创建一个1秒超时的上下文来处理剩余任务
	drainCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// 尝试处理剩余的任务
	for {
		select {
		case <-drainCtx.Done():
			// 超时，放弃剩余任务
			return
		case task, ok := <-l.tasks:
			if !ok {
				return
			}
			// 尝试执行剩余任务
			_ = task(ctx)
		default:
			// 没有更多任务
			return
		}
	}
}

// Stop 停止任务循环，并等待所有正在处理的任务完成
func (l *TaskLoop) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.running = false
	close(l.stopCh)
	l.mu.Unlock()

	<-l.doneCh // 等待处理完成
}

// TryAddTask 尝试添加任务，如果队列满则立即返回错误
func (l *TaskLoop) TryAddTask(task Task) error {
	select {
	case <-l.stopCh:
		return context.Canceled
	case l.tasks <- task:
		return nil
	default:
		return errors.New("任务队列已满")
	}
}

// IsRunning 返回循环是否正在运行
func (l *TaskLoop) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// QueuedTasksCount 返回当前队列中等待的任务数量
func (l *TaskLoop) QueuedTasksCount() int {
	return len(l.tasks)
}
