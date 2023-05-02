package main

import (
	"context"
	"log"
	"time"

	"github.com/davidwashere/waiter/waiter"
)

var wm *waiter.Manager[string]

func main() {
	log.Println("start")

	// the manager manages and coordinates between waiters and sendors
	wm = waiter.NewManager[string]()

	// manager must be started in order to process messages
	wm.Start(context.Background())

	// a successfully created waiter will have a unique ID
	w, err := wm.NewWaiter()
	check(err)

	doAsyncTask(w.ID())

	// wait for async task to send a respone to the waiter manager
	r, err := w.Wait(context.Background())
	check(err)
	log.Printf("result: %v\n", r)
}

func doAsyncTask(id string) {
	go func() {
		time.Sleep(2 * time.Second)
		wm.Send(id, "done", nil)
	}()
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
