package wrapper

type Loop struct {
	// Task is the function that will be called in a loop
	Task func() error
	Done chan struct{}
}

func (l *Loop) Close() {
	close(l.Done)
}

func NewLoop() *Loop {
	l := &Loop{
		Done: make(chan struct{}),
	}

	go l.runLoop()
	return l
}

func (l *Loop) runLoop() {
	for {
		select {
		case <-l.Done:
			return
		default:
			if err := l.Task(); err != nil {
				return
			}
		}
	}
}
