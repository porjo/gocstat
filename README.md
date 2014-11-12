## Gocstat

[![godoc/gocstat](https://godoc.org/github.com/porjo/gocstat?status.png)](https://godoc.org/github.com/porjo/gocstat)

gocstat is a Go library for reading selected statistics about Linux containers.

Running containers are discovered by walking `BasePath` periodically
Containers stopped or removed from the system are automatically pruned
from the list of discovered containers.

The following example shows how to initalize the package and poll
statistics in a for loop:

```Go
errChan := make(chan error)
err := gocstat.Init(errChan)
if err != nil {
	log.Fatal(err)
}
go func() {
	for {
		time.Sleep(1 * time.Second)
		stats, err := gocstat.ReadStats()
		if err != nil {
			log.Fatal(err)
		}
		for containerId, stat := range stats {
			 // stat.Memory.RSS
			 // stat.Memory.Cache
			 // stat.CPU.User
			 // stat.CPU.System
		}
	}
}()
// block waiting for channel
err = <-errChan
if err != nil {
	fmt.Printf("errChan %s\n", err)
}
```
