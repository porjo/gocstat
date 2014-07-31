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
//			stats, err := gocstat.ReadStats()
//			if err != nil {
//				errChan <- err
//			}
//			for containerId, stat := range stats {
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
	h                  *holder
)

type holder struct {
	sync.Mutex
	containerStats Stats
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
	h = &holder{}
	h.containerStats = make(Stats)
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
	h.Lock()
	defer h.Unlock()

	if err := filepath.Walk(path, walkFn); err != nil {
		return fmt.Errorf("error walking path '%s', err %s", path, err)
	}
	return nil
}

// Retrieve current container statistics.
func ReadStats() (Stats, error) {
	if h == nil {
		return nil, fmt.Errorf("not initialized")
	}
	h.Lock()
	defer h.Unlock()
	for id, cs := range h.containerStats {
		if cs.Memory.path != "" {
			b, err := readFile(cs.Memory.path, id)
			if err != nil {
				return nil, err
			}
			h.containerStats[id].Memory.create(string(b))
		}
		if cs.CPU.path != "" {
			b, err := readFile(cs.CPU.path, id)
			if err != nil {
				return nil, err
			}
			h.containerStats[id].CPU.create(string(b))
		}
		if cs.BlkIO.Bytes.path != "" {
			b, err := readFile(cs.BlkIO.Bytes.path, id)
			if err != nil {
				return nil, err
			}
			h.containerStats[id].BlkIO.Bytes.create(string(b))
		}
		if cs.BlkIO.IOPS.path != "" {
			b, err := readFile(cs.BlkIO.IOPS.path, id)
			if err != nil {
				return nil, err
			}
			h.containerStats[id].BlkIO.IOPS.create(string(b))
		}
	}
	return h.containerStats, nil
}

func readFile(path, id string) ([]byte, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			delete(h.containerStats, id)
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
		if _, ok := h.containerStats[id]; !ok {
			h.containerStats[id] = &ContainerStats{}
		}
	} else {
		if _, ok := h.containerStats[id]; ok {
			baseName := path.Base(info.Name())
			switch baseName {
			case memFile:
				h.containerStats[id].Memory.path = filePath
			case cPUFile:
				h.containerStats[id].CPU.path = filePath
			case blkIOFile:
				h.containerStats[id].BlkIO.Bytes.path = filePath
			case blkIOBytesFile:
				h.containerStats[id].BlkIO.IOPS.path = filePath
			}
		}
	}

	return nil
}
