// gocstat reads selected statistics about Linux containers
// e.g. CPU & memory usage
// BasePath is automatically polled to discover containers.
// Calls to ReadCgroups returns statistics for the discovered containers.
// Containers that are removed from the system are automatically pruned
// from the list of discovered containers

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

var updateInterval = time.Duration(5 * time.Second)

// Directory to start search
var BasePath = "/sys/fs/cgroup"

// Read directories which match this regex
var ContainerDirRegexp = `.*docker-([0-9a-z]{64})\.scope.*`

var MemFile = "memory.stat"
var CPUFile = "cpuacct.stat"

var re *regexp.Regexp

var cg *cgroups

type cgroups struct {
	sync.Mutex
	Containers map[string]*ContainerStat
}

type ContainerStat struct {
	Memory CMemStat
	CPU    CCPUStat
}

type CCPUStat struct {
	path   string
	User   uint64 `json:"user"`
	System uint64 `json:"system"`
}

type CMemStat struct {
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

// Launches go routine to periodically scan BasePath for containers
// Supply optional errChan for reporting errors
func InitCgroups(errChan chan<- error) error {
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

// Get current container readings. Returns a map of container IDs and
// corresponding ContainerStat
func ReadCgroups() (map[string]*ContainerStat, error) {
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
