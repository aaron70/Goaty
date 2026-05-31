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
	clearScreen()
	fmt.Println(pool.Metrics())
	// fmt.Printf("Time: %s\n", time.Now())
	// fmt.Println("Pool metrics:")
	// fmt.Printf("\tPool Done: %t\n", pool.IsDone())
	// fmt.Println("Tasks:")
	// fmt.Printf("\tEnqueued: %d\n", metrics.Tasks.Enqueued)
	// fmt.Printf("\tConsumed: %d\n", metrics.Tasks.Consumed)
	// fmt.Printf("\tDone: %d\n", metrics.Tasks.Done)
	// fmt.Printf("\tFailed: %d\n", metrics.Tasks.Failed)
	// fmt.Println("Workers:")
	// fmt.Printf("\tCreated: %d\n", metrics.Workers.Created)
	// fmt.Printf("\tAlive: %d\n", metrics.Workers.Alive)
	// fmt.Printf("\tRunning: %d\n", metrics.Workers.Running)
	// fmt.Printf("\tIdle: %d\n", metrics.Workers.Idle)
	// fmt.Printf("\tFailed: %d\n", metrics.Workers.Failed)
	// fmt.Printf("\tDone: %d\n", metrics.Workers.Done)
}

func main() {
	start := time.Now()
	var wg sync.WaitGroup
	defer fmt.Println("Program finished...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	n := 1_000_000

	work := func(ctx context.Context, task int) (int, error) {
		// fmt.Printf("Working on task: %d\n", task)
		// time.Sleep(time.Millisecond * 2)
		return task, nil
	}

	pool := utils.Must(concurrency.NewPoolResult(ctx, work,
		concurrency.NewPoolResultWithBufferSize(n),
		concurrency.NewPoolResultWithKeepAlive(true),
		// concurrency.NewPoolResultWithMaxWorkers(300),
		// concurrency.NewPoolResultWithMinWorkers(150),
		// concurrency.NewPoolResultWithIdleDuration(time.Second * 3),
	))

	wg.Go(func() {
		printMetrics(pool)
		defer printMetrics(pool)
		for {
			select {
			case <-time.Tick(time.Millisecond):
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
		res, errs := pool.ResultsErr()
		channels.Drain(ctx, res)
		channels.Drain(ctx, errs)
	})


	utils.PanicErr(pool.ProduceTasks(n, func(index int) int { time.Sleep(time.Second * 0); return index }))
	pool.Close()

	fmt.Println("Waiting for pool...")
	pool.Wait()
	fmt.Println("Waiting for main goroutines...")
	wg.Wait()
	fmt.Printf("Took: %s\n", time.Since(start))
}

// func main() {
// 	start := time.Now()
// 	// defer fmt.Println("Program Finished...")
//
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
// 	defer cancel()
//
// 	var wg sync.WaitGroup
//
// 	n := 1000
// 	tasks := make(chan int, n)
//
// 	work := func(ctx context.Context, task int) (int, error) {
// 		// fmt.Printf("Working on task: %d\n", task)
// 		time.Sleep(time.Millisecond * 100)
// 		return task, nil
// 	}
//
// 	pool := utils.Must(concurrency.NewPoolResult(ctx, work,
// 		concurrency.NewPoolResultWithBufferSize(0),
// 		concurrency.NewPoolResultWithMaxWorkers(100),
// 		// concurrency.NewPoolResultWithIdleDuration(time.Millisecond),
// 	))
//
// 	wg.Go(func() {
// 		printMetrics(pool)
// 		defer printMetrics(pool)
// 		for {
// 			select {
// 			case <-time.Tick(time.Millisecond):
// 				printMetrics(pool)
// 				if pool.IsDone() {
// 					return
// 				}
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	})
//
// 	wg.Go(func() {
// 		res, errs := pool.ResultsErr()
// 		channels.Drain(ctx, res)
// 		channels.Drain(ctx, errs)
// 	})
//
// 	wg.Go(func() {
// 		for i := range n {
// 			tasks <- i
// 			// time.Sleep(time.Millisecond * 20)
// 		}
// 		close(tasks)
// 	})
//
// 	utils.LogDefaultErr(pool.RecvTasks(tasks))
// 	pool.Close()
//
// 	// utils.LogDefaultErr(pool.Wait())
//
// 	// fmt.Println("Waiting for main goroutines")
// 	wg.Wait()
// 	fmt.Printf("Time elapsed: %s\n", time.Since(start))
// }
