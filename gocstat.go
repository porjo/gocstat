// gocstat reads selected statistics about Linux containers.
//
// Containers are discovered by walking BasePath periodically.
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
//		defer close(errChan)
//		for {
//			time.Sleep(1 * time.Second)
//			containers, err := gocstat.ReadStats()
//			if err != nil {
//				errChan <- err
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

const (
	memFile = "memory.stat"
	cPUFile = "cpuacct.stat"
)

var (
	namesUpdateInterval = time.Duration(30 * time.Second)
	// Directory to start search
	BasePath = "/sys/fs/cgroup"
	// Process directories which match this regex. The section enclosed in parentheses
	// will be used as the container ID
	ContainerDirRegexp = `.*docker-([0-9a-z]{64})\.scope.*`
	re                 *regexp.Regexp
	cg                 *cgroups
)

type cgroups struct {
	sync.Mutex
	Containers Stats
}

type ContainerStats struct {
	Memory MemStat
	CPU    CPUStat
	BlkIO  BlkIOStat
}

// map key corresponds with the container ID.
type Stats map[string]*ContainerStats

type commonFields struct {
	path      string
	Timestamp time.Time `json:"timestamp"`
}

type CPUStat struct {
	commonFields
	User   uint64 `json:"user"`
	System uint64 `json:"system"`
}

type MemStat struct {
	commonFields
	RSS   uint64 `json:"rss"`
	Cache uint64 `json:"cache"`
}

func (c *CPUStat) create(content string) {

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
			c.User, _ = strconv.ParseUint(fields[1], 10, 64)
		case 1:
			c.System, _ = strconv.ParseUint(fields[1], 10, 64)
		default:
			break
		}
	}
	c.Timestamp = time.Now()
}

func (m *MemStat) create(content string) {

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
			m.Cache, _ = strconv.ParseUint(fields[1], 10, 64)
		case 1:
			m.RSS, _ = strconv.ParseUint(fields[1], 10, 64)
		default:
			break
		}
	}
	m.Timestamp = time.Now()
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
	cg.Containers = make(Stats)
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
			time.Sleep(namesUpdateInterval)
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
		return fmt.Errorf("error walking path '%s', err %s", path, err)
	}
	return nil
}

// Retrieve current container statistics.
func ReadStats() (Stats, error) {
	if cg == nil {
		return nil, fmt.Errorf("not initialized")
	}
	cg.Lock()
	defer cg.Unlock()
	for id, cs := range cg.Containers {
		if cs.Memory.path != "" {
			b, err := readFile(cs.Memory.path, id)
			if err != nil {
				return nil, err
			}
			cg.Containers[id].Memory.create(string(b))
		}
		if cs.CPU.path != "" {
			b, err := readFile(cs.CPU.path, id)
			if err != nil {
				return nil, err
			}
			cg.Containers[id].CPU.create(string(b))
		}
		if cs.BlkIO.Bytes.path != "" {
			b, err := readFile(cs.BlkIO.Bytes.path, id)
			if err != nil {
				return nil, err
			}
			cg.Containers[id].BlkIO.Bytes.create(string(b))
		}
		if cs.BlkIO.IOPS.path != "" {
			b, err := readFile(cs.BlkIO.IOPS.path, id)
			if err != nil {
				return nil, err
			}
			cg.Containers[id].BlkIO.IOPS.create(string(b))
		}
	}
	return cg.Containers, nil
}

func readFile(path, id string) ([]byte, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("removing old container id %s\n", id)
			delete(cg.Containers, id)
			return []byte{}, nil
		}
		return nil, err
	}
	return b, nil
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
			cg.Containers[id] = &ContainerStats{}
		}
	} else {
		if _, ok := cg.Containers[id]; ok {
			baseName := path.Base(info.Name())
			switch baseName {
			case memFile:
				cg.Containers[id].Memory.path = filePath
			case cPUFile:
				cg.Containers[id].CPU.path = filePath
			case blkIOFile:
				cg.Containers[id].BlkIO.Bytes.path = filePath
			case blkIOBytesFile:
				cg.Containers[id].BlkIO.IOPS.path = filePath
			}
		}
	}

	return nil
}
