package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduler = struct {
	cron     *cron.Cron
	executor *executor
	jobs     map[string]cron.EntryID
	mu       sync.Mutex
}{
	cron:     cron.New(cron.WithSeconds()), // 支持秒级 cron 表达式
	executor: &executor{},
	jobs:     make(map[string]cron.EntryID),
}

type Task interface {
	Run(ctx context.Context) error
}

type Job struct {
	Name      string        // 任务名称
	Spec      string        // cron 表达式
	Task      Task          // 任务
	Retry     int           // 重试次数
	Timeout   time.Duration // 超时时间
	Singleton bool          // 是否禁止并发
}

func init() {
	scheduler.cron.Start()
}

// AddJob 添加一个定时任务。
//
// 使用方式:
//  1. 实现 Task 接口, 在 Run 方法中编写任务逻辑。
//  2. 构造 Job, Name 用作任务唯一标识, Spec 使用支持秒级的 cron 表达式,
//     Task 传入任务实例, Retry 设置失败后的重试次数, Timeout 设置单次执行超时时间,
//     Singleton 设置为 true 时会跳过上一次尚未结束的执行, 避免同一任务并发运行。
//  3. 调用 AddJob(job) 注册任务, 注册成功后任务会由全局 scheduler 自动调度执行。
//
// 示例:
//
//	err := scheduler.AddJob(&scheduler.Job{
//		Name:      "sync-device-status",
//		Spec:      "0 */5 * * * *",
//		Task:      task,
//		Retry:     3,
//		Timeout:   time.Minute,
//		Singleton: true,
//	})
func AddJob(job *Job) error {

	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()

	var wrapper func()

	// 是否单例执行
	if job.Singleton {
		wrapper = cron.SkipIfStillRunning(cron.DefaultLogger)(
			cron.FuncJob(func() {
				scheduler.executor.execute(job)
			}),
		).Run
	} else {
		wrapper = func() {
			scheduler.executor.execute(job)
		}
	}

	id, err := scheduler.cron.AddFunc(job.Spec, wrapper)
	if err != nil {
		return err
	}

	scheduler.jobs[job.Name] = id
	return nil
}

func Remove(name string) {
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()

	if id, ok := scheduler.jobs[name]; ok {
		scheduler.cron.Remove(id)
		delete(scheduler.jobs, name)
	}
}

func Stop(ctx context.Context) error {
	stopCtx := scheduler.cron.Stop()

	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
