package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aaron70/goaty/channels"
	"github.com/aaron70/goaty/utils"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	bc := utils.Must(channels.NewBatchCollector[int](25, 500*time.Millisecond))

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case batch, open := <-bc.C:
				if !open { return }
				fmt.Printf("[consumer] Received batch: %v\n", batch)
			}
		}
	})

	for i := 1; i <= 1_000_000; i++ {
		bc.Add(ctx, i)
		// fmt.Printf("[producer] Added %d\n", i)
	}
	bc.Close()

	// bc.Flush(ctx)
	// fmt.Println("[producer] Flushed remaining")

	wg.Wait()
}
