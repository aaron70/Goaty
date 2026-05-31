package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/aaron70/goaty/concurrency"
	"github.com/aaron70/goaty/utils"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func printMetrics[T, R any](pool *concurrency.PoolResult[T, R]) {
	metrics := pool.Metrics()
	clearScreen()
	fmt.Printf("Time: %s\n", time.Now())
	fmt.Println("Pool metrics:")
	fmt.Printf("\tPool Done: %t\n", pool.IsDone())
	fmt.Println("Tasks:")
	fmt.Printf("\tEnqueued: %d\n", metrics.Tasks.Enqueued)
	fmt.Printf("\tConsumed: %d\n", metrics.Tasks.Consumed)
	fmt.Printf("\tDone: %d\n", metrics.Tasks.Done)
	fmt.Printf("\tFailed: %d\n", metrics.Tasks.Failed)
	fmt.Println("Workers:")
	fmt.Printf("\tCreated: %d\n", metrics.Workers.Created)
	fmt.Printf("\tAlive: %d\n", metrics.Workers.Alive)
	fmt.Printf("\tRunning: %d\n", metrics.Workers.Running)
	fmt.Printf("\tIdle: %d\n", metrics.Workers.Idle)
	fmt.Printf("\tFailed: %d\n", metrics.Workers.Failed)
	fmt.Printf("\tDone: %d\n", metrics.Workers.Done)
}

func main() {
	// defer fmt.Println("Program Finished...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	var wg sync.WaitGroup

	n := 70
	tasks := make(chan int, n)

	pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task int) (int, error) {
		// fmt.Printf("Working on task: %d\n", task)
		time.Sleep(time.Millisecond * 1500)
		return task, nil
	}))

	wg.Go(func() {
		printMetrics(pool)
		for {
			select {
			case <-time.Tick(time.Millisecond * 100):
				printMetrics(pool)
				if pool.IsDone() {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})

	wg.Go(func() {
		for i := range n {
			tasks <- i
			time.Sleep(time.Millisecond * 100)
		}
		close(tasks)
	})

	utils.LogDefaultErr(pool.RecvTasks(tasks))
	pool.Close()

	// utils.LogDefaultErr(pool.Wait())

	res, errs := pool.ResultsErr()
	channels.Drain(ctx, res)
	channels.Drain(ctx, errs)

	wg.Wait()
}
