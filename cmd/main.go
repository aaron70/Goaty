package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aaron70/goaty/concurrency"
	"github.com/aaron70/goaty/utils"
)

func main() {
	defer fmt.Println("Finished")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	n := 1
	tasks := make(chan int, n)
	for i := range n {
		tasks <- i + 1
	}
	close(tasks)

	pool := utils.Must(concurrency.NewPoolResult(ctx, func(ctx context.Context, task int) (int, error) {
		fmt.Printf("Working on task: %d\n", task)
		return task, nil
	}))
	pool.RecvTasks(tasks)
	pool.Close()

	pool.Wait()
}
