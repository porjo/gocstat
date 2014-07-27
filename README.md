## Gocstat

[![godoc/gocstat](https://godoc.org/github.com/porjo/gocstat?status.png)](https://godoc.org/github.com/porjo/gocstat)

gocstat reads selected statistics about Linux containers.

Containers are discovered by walking `BasePath` periodically
Containers removed from the system are automatically pruned
from the list of discovered containers.

The following example shows how to initalize the package and poll
statistics in a for loop:

```Go
errChan := make(chan error)
err := linuxproc.Init(errChan)
if err != nil {
	log.Fatal(err)
}
go func() {
	for {
		time.Sleep(1 * time.Second)
		containers, err := linuxproc.ReadStats()
		if err != nil {
			log.Fatal(err)
		}
		for containerId, stat := range containers {
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
