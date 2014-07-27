package main

import (
	"fmt"
	"log"
	"time"

	linuxproc "github.com/porjo/gocstat"
)

func main() {
	errChan := make(chan error)
	err := linuxproc.InitCgroups(errChan)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			time.Sleep(1 * time.Second)
			containers, err := linuxproc.ReadCgroups()
			if err != nil {
				log.Fatal(err)
			}
			for id, stat := range containers {
				fmt.Printf("id %s stat %v\n", id, stat)
			}
		}
	}()
	err = <-errChan
	if err != nil {
		fmt.Printf("errChan %s\n", err)
	}
}
