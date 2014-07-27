// gocstat reads selected statistics about Linux containers.
//
// Containers are discovered by walking BasePath periodically
// Containers removed from the system are automatically pruned
// from the list of discovered containers.
//
// The following example shows how to initalize the package and poll
// statistics in a for loop:
//
//	errChan := make(chan error)
//	err := gocstat.Init(errChan)
//	if err != nil {
//		log.Fatal(err)
//	}
//	go func() {
//		for {
//			time.Sleep(1 * time.Second)
//			containers, err := gocstat.ReadStats()
//			if err != nil {
//				log.Fatal(err)
//			}
//			for containerId, stat := range containers {
//				// stat.Memory.RSS
//				// stat.Memory.Cache
//				// stat.CPU.User
//				// stat.CPU.System
//			}
//		}
//	}()
//	// block waiting for channel
//	err = <-errChan
//	if err != nil {
//		fmt.Printf("errChan %s\n", err)
//	}
//
package gocstat

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MemFile = "memory.stat"
const CPUFile = "cpuacct.stat"

var updateInterval = time.Duration(30 * time.Second)

// Directory to start search
var BasePath = "/sys/fs/cgroup"

// Process directories which match this regex. The section enclosed in parentheses
// will be used as the container ID
var ContainerDirRegexp = `.*docker-([0-9a-z]{64})\.scope.*`

var re *regexp.Regexp

var cg *cgroups

type cgroups struct {
	sync.Mutex
	Containers map[string]*ContainerStat
}

type ContainerStat struct {
	Memory MemStat
	CPU    CPUStat
}

type CPUStat struct {
	path   string
	User   uint64 `json:"user"`
	System uint64 `json:"system"`
}

type MemStat struct {
	path  string
	RSS   uint64 `json:"rss"`
	Cache uint64 `json:"cache"`
}

func (cs *ContainerStat) createCPUStat(content string) {

	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return
	}
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch i {
		case 0:
			cs.CPU.User, _ = strconv.ParseUint(fields[1], 10, 64)
		case 1:
			cs.CPU.System, _ = strconv.ParseUint(fields[1], 10, 64)
		default:
			break
		}
	}
}

func (cs *ContainerStat) createMemStat(content string) {

	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return
	}
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch i {
		case 0:
			cs.Memory.Cache, _ = strconv.ParseUint(fields[1], 10, 64)
		case 1:
			cs.Memory.RSS, _ = strconv.ParseUint(fields[1], 10, 64)
		default:
			break
		}
	}
}

// Init initalizes the package and must be run before ReadStats().
// A goroutine is launched to periodically scan BasePath for containers.
// errChan is optional and used by the goroutine for reporting any errors.
func Init(errChan chan<- error) error {
	var err error
	re, err = regexp.Compile(ContainerDirRegexp)
	if err != nil {
		return err
	}
	cg = &cgroups{}
	cg.Containers = make(map[string]*ContainerStat)
	go func() {
		for {
			err := updatePaths(BasePath)
			if err != nil && errChan != nil {
				select {
				case errChan <- err:
				default:
				}
				close(errChan)
				return
			}
			time.Sleep(updateInterval)
		}
	}()
	return nil
}

func updatePaths(path string) error {
	if cg == nil {
		return fmt.Errorf("not initialized")
	}

	cg.Lock()
	defer cg.Unlock()

	if err := filepath.Walk(path, walkFn); err != nil {
		return fmt.Errorf("error walking path '%s', err %s", err)
	}
	return nil
}

// Retrieve current container statistics. The map key corresponds with the container ID.
func ReadStats() (map[string]*ContainerStat, error) {
	if cg == nil {
		return nil, fmt.Errorf("not initialized")
	}
	cg.Lock()
	defer cg.Unlock()
	for id, cs := range cg.Containers {
		if cs.Memory.path != "" {
			b, err := ioutil.ReadFile(cs.Memory.path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("removing old container id %s\n", id)
					delete(cg.Containers, id)
					continue
				}
				return nil, err
			}
			cg.Containers[id].createMemStat(string(b))
		}
		if cs.CPU.path != "" {
			b, err := ioutil.ReadFile(cs.CPU.path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("removing old container id %s\n", id)
					delete(cg.Containers, id)
					continue
				}
				return nil, err
			}
			cg.Containers[id].createCPUStat(string(b))
		}
	}
	return cg.Containers, nil
}

func walkFn(filePath string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	matches := re.FindStringSubmatch(filePath)
	if len(matches) < 2 {
		return nil
	}
	id := matches[1]
	if info.IsDir() {
		if _, ok := cg.Containers[id]; !ok {
			cg.Containers[id] = &ContainerStat{}
		}
	} else {
		if _, ok := cg.Containers[id]; ok {
			if path.Base(info.Name()) == MemFile {
				cg.Containers[id].Memory.path = filePath
			}
			if path.Base(info.Name()) == CPUFile {
				cg.Containers[id].CPU.path = filePath
			}
		}
	}

	return nil
}
