package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/davidwashere/waiter/waiter"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	wm := waiter.NewManager[string]()
	wm.Start(context.Background())

	wg := sync.WaitGroup{}

	var ids []string

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		w, err := wm.NewWaiter()
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, w.ID())

		go func(w waiter.Waiter[string]) {
			defer wg.Done()
			w.Wait(ctx)
		}(w)
	}

	fmt.Println(ids)

	ids = ids[:len(ids)/2]

	for _, id := range ids {
		wm.Send(id, "howdy", nil)
	}

	wg.Wait()
	time.Sleep(1 * time.Second)
}
