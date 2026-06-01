package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/aaron70/goaty/concurrency"
	"github.com/aaron70/goaty/utils"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func printMetrics[T, R any](names []string, pools ...*concurrency.PoolResult[T, R]) {
	clearScreen()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	row := func(label string, values []string) {
		fmt.Fprintf(w, "%s\t%s\n", label, strings.Join(values, "\t"))
	}

	getMetric := func(fn func(*concurrency.PoolResult[T, R]) any) []string {
		s := make([]string, len(pools))
		for i, pool := range pools {
			s[i] = fmt.Sprintf("%v", fn(pool))
		}
		return s
	}

	// Header
	fmt.Fprintf(w, "Metric\t%s\n", strings.Join(names, "\t"))
	fmt.Fprintf(w, "------\t%s\n", strings.Join(make([]string, len(names)), "\t"))
	row("Queue is Done: ", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.IsDone() }))
	row("Queue accpets tasks: ", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.AcceptsTasks() }))
	fmt.Fprintf(w, "------\t%s\n", strings.Join(make([]string, len(names)), "\t"))

	// Task rows
	row("Tasks Enqueued", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Tasks.Enqueued }))
	row("Tasks Consumed", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Tasks.Consumed }))
	row("Tasks Done", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Tasks.Done }))
	row("Tasks Failed", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Tasks.Failed }))
	fmt.Fprintf(w, "------\t%s\n", strings.Join(make([]string, len(names)), "\t"))

	// Worker rows
	row("Workers Created", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Workers.Created }))
	row("Workers Alive", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Workers.Alive }))
	row("Workers Running", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Workers.Running }))
	row("Workers Idle", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Workers.Idle }))
	row("Workers Done", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Workers.Done }))
	row("Workers Failed", getMetric(func(p *concurrency.PoolResult[T, R]) any { return p.Metrics().Workers.Failed }))

	w.Flush()
}

func main() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	sleep := time.Second * 2

	n := 1
	buffer := 0

	producerPool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task int) (int, error) {
		time.Sleep(sleep)
		return task, nil
	}))

	consumer1Pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task int) (int, error) {
		time.Sleep(sleep)
		return task, nil
	}))

	consumer2Pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task int) (int, error) {
		time.Sleep(sleep)
		return task, nil
	}))

	consumer3Pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task int) (int, error) {
		time.Sleep(sleep)
		return task, nil
	}))

	p, perrs := producerPool.ResultsErr()
	c1, c1errs := consumer1Pool.ResultsErr()
	c2, c2errs := consumer1Pool.ResultsErr()
	c3, c3errs := consumer1Pool.ResultsErr()
	errs := channels.Merge(ctx, buffer, perrs, c1errs, c2errs, c3errs)

	wg.Go(func() {
		producerPool.ProduceTasks(n, func(index int) int {
			time.Sleep(sleep)
			return index
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

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.Tick(time.Millisecond * 500):
				printMetrics([]string{"Producer", "consumer1", "consumer2", "consumer3"}, producerPool, consumer1Pool, consumer2Pool, consumer3Pool)
			}
		}
	})

	wg.Go(func() {
		for {
			err, open, errCtx := channels.Recv(ctx, errs)
			utils.PanicErr(errCtx)
			utils.PanicErr(err)
			if !open {
				break
			}
		}
	})

	producerPool.Wait()
	consumer1Pool.Wait()
	consumer2Pool.Wait()
	consumer3Pool.Wait()
	fmt.Println("Waiting for goroutines")
	wg.Wait()
}
