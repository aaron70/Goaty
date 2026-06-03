package main

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aaron70/decoy"
	"github.com/aaron70/goaty/channels"
	"github.com/aaron70/goaty/concurrency"
	concurrencyv2 "github.com/aaron70/goaty/concurrency/v2"
	"github.com/aaron70/goaty/utils"
)

func main() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// sleep := time.Microsecond * 2

	n := 30000
	// buffer := n
	// maxWorkers := 200

	d := utils.Must(decoy.NewDecoyWithSeed(0))
	templateCompiled := utils.Must(d.CompileTemplate(`{ "id": {{ NextIncrementalInt "id" 0 1}}, "text": "{{ RandomText 50 }}" }`,
		decoy.WithTemplateNamed("template"),
	))

	pool := utils.Must(concurrencyv2.NewPool(ctx, func(ctx context.Context, task string) (string, error) {
		buffer := new(bytes.Buffer)
		err := templateCompiled.Execute(buffer, "")
		if err != nil {
			return "", err
		}
		return buffer.String(), nil
	},
		// concurrencyv2.NewPoolWithMinWorkers(0),
		concurrencyv2.NewPoolWithMaxWorkers(1),
		concurrencyv2.NewPoolWithBufferSize(0),
		// concurrencyv2.NewPoolWithKeepAlive(true),
		// concurrencyv2.NewPoolWithIdleDuration(time.Millisecond * 100),
	))

	done := make(chan struct{})
	wg.Go(func() {
		defer concurrencyv2.PrintPools([]string{"pool"}, pool)
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-time.Tick(time.Millisecond * 500):
				concurrencyv2.PrintPools([]string{"pool"}, pool)
			}
		}
	})

	wg.Go(func() {
		res, errs := pool.ResultsErr()
		wg.Go(func() {
			channels.Drain(ctx, res)
		})
		wg.Go(func() {
			channels.Drain(ctx, errs)
		})
	})

	tasks := make(chan string, n)
	wg.Go(func() {
		for i := range n {
			// time.Sleep(time.Millisecond * 1000)
			tasks <- fmt.Sprintf("%d", i)
		}
		close(tasks)
	})

	utils.PanicErr(pool.RecvTasks(tasks))
	pool.Close()

	pool.Wait()
	close(done)
	wg.Wait()
}

func main2() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	sleep := time.Microsecond * 1000 * 1

	n := 3000
	buffer := n
	maxWorkers := 200

	producerPool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task string) (string, error) {
		return decoy.Default.ParseTemplateString(`{ "id": {{ NextIncrementalInt "id" 0 1 }}, "text": "{{ RandomText 100 }}" }`)
	},
		concurrency.NewPoolResultWithBufferSize(buffer),
		concurrency.NewPoolResultWithMaxWorkers(maxWorkers),
	))

	consumer1Pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, template string) (string, error) {
		return decoy.Default.ParseTemplateString(`echo "{{ .Template }}"`,
			decoy.WithData(map[string]any{"Template": template}),
		)
	},
		concurrency.NewPoolResultWithBufferSize(buffer),
		concurrency.NewPoolResultWithMaxWorkers(maxWorkers),
	))

	consumer2Pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task string) (string, error) {
		time.Sleep(sleep)
		return task, nil
	},
		concurrency.NewPoolResultWithBufferSize(buffer),
		concurrency.NewPoolResultWithMaxWorkers(maxWorkers),
	))

	consumer3Pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task string) (string, error) {
		time.Sleep(sleep)
		return decoy.Default.ParseTemplateString(`Written: {{ . }}`, decoy.WithData(task))
	},
		concurrency.NewPoolResultWithBufferSize(buffer),
		concurrency.NewPoolResultWithMaxWorkers(maxWorkers),
	))

	p, perrs := producerPool.ResultsErr()
	c1, c1errs := consumer1Pool.ResultsErr()
	c2, c2errs := consumer2Pool.ResultsErr()
	c3, c3errs := consumer3Pool.ResultsErr()
	errs := channels.Merge(ctx, buffer, perrs, c1errs, c2errs, c3errs)

	wg.Go(func() {
		producerPool.ProduceTasks(n, func(index int) string {
			// time.Sleep(sleep)
			return fmt.Sprintf("%d", index)
		})
		producerPool.Close()
	})

	wg.Go(func() {
		consumer1Pool.RecvTasks(p)
		consumer1Pool.Close()
	})

	wg.Go(func() {
		consumer2Pool.RecvTasks(c1)
		consumer2Pool.Close()
	})

	wg.Go(func() {
		consumer3Pool.RecvTasks(c2)
		consumer3Pool.Close()
	})

	wg.Go(func() {
		channels.Drain(ctx, c3)
	})

	done := make(chan struct{})
	wg.Go(func() {
		defer concurrency.PrintPool([]string{"Producer", "consumer1", "consumer2", "consumer3"}, producerPool, consumer1Pool, consumer2Pool, consumer3Pool)
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-time.Tick(time.Millisecond * 500):
				concurrency.PrintPool([]string{"Producer", "consumer1", "consumer2", "consumer3"}, producerPool, consumer1Pool, consumer2Pool, consumer3Pool)
			}
		}
	})

	for {
		err, open, errCtx := channels.Recv(ctx, errs)
		utils.PanicErr(errCtx)
		utils.PanicErr(err)
		if !open {
			break
		}
	}
	close(done)

	producerPool.Wait()
	consumer1Pool.Wait()
	consumer2Pool.Wait()
	consumer3Pool.Wait()
	fmt.Println("Waiting for goroutines")
	wg.Wait()
}
