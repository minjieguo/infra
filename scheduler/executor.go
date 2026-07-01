package scheduler

import (
	"context"
	"fmt"
)

type executor struct{}

func (e *executor) execute(job *Job) {
	ctx := context.Background()

	// 超时控制
	if job.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, job.Timeout)
		defer cancel()
	}

	for i := 0; i <= job.Retry; i++ {
		err := run(ctx, job)

		if err == nil {
			return
		}

		// log.Printf("job %s failed: %v (retry %d)", job.Name, err, i)
	}
}

func run(ctx context.Context, job *Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return job.Task.Run(ctx)
}
