// Copyright (C) 2014 Ian Bishop
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, write to the Free Software Foundation, Inc.,
// 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.

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
	//	"log"
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
	// Directory to start search
	BasePath = "/sys/fs/cgroup"

	// Process directories which match this regex. The section enclosed in parentheses
	// will be used as the container ID
	ContainerDirRegexp = `.*docker-([0-9a-z]{64})\.scope.*`

	re                  *regexp.Regexp
	statsHolder         *holder
	namesUpdateInterval = time.Duration(30 * time.Second)
)

type holder struct {
	sync.Mutex
	containers Cmap
}

type Cstats struct {
	Memory MemStat
	CPU    CPUStat
	BlkIO  BlkIOStat
}

// Map key corresponds with the container ID.
//
// Map value must be a pointer to a struct, see:
// https://code.google.com/p/go/issues/detail?id=3117
type Cmap map[string]*Cstats

type stat interface {
	create(content string)
}

type CPUStat struct {
	User      uint64
	System    uint64
	path      string
	Timestamp time.Time
}

type MemStat struct {
	RSS       uint64
	Cache     uint64
	path      string
	Timestamp time.Time
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
	statsHolder = &holder{}
	statsHolder.containers = make(Cmap)
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
	statsHolder.Lock()
	defer statsHolder.Unlock()

	if err := filepath.Walk(path, walkFn); err != nil {
		return fmt.Errorf("error walking path '%s', err %s", path, err)
	}
	return nil
}

// Retrieve current container statistics.
func ReadStats() (Cmap, error) {
	if statsHolder == nil {
		return nil, fmt.Errorf("not initialized")
	}
	statsHolder.Lock()
	defer statsHolder.Unlock()
	for id, cs := range statsHolder.containers {
		if cs.Memory.path != "" {
			b, err := readFile(cs.Memory.path)
			if err != nil {
				if os.IsNotExist(err) {
					delete(statsHolder.containers, id)
					continue
				}
				return nil, err
			}
			statsHolder.containers[id].Memory.create(string(b))
		}
		if cs.CPU.path != "" {
			b, err := readFile(cs.CPU.path)
			if err != nil {
				if os.IsNotExist(err) {
					delete(statsHolder.containers, id)
					continue
				}
				return nil, err
			}
			statsHolder.containers[id].CPU.create(string(b))
		}
		if cs.BlkIO.Bytes.path != "" {
			//err := readFile(cs.BlkIO.Bytes.path, id, &statsHolder.containers[id].BlkIO.Bytes)
			b, err := readFile(cs.BlkIO.Bytes.path)
			if err != nil {
				if os.IsNotExist(err) {
					delete(statsHolder.containers, id)
					continue
				}
				return nil, err
			}
			statsHolder.containers[id].BlkIO.Bytes.create(string(b))
		}
		if cs.BlkIO.IOPS.path != "" {
			//err := readFile(cs.BlkIO.IOPS.path, id, &statsHolder.containers[id].BlkIO.IOPS)
			b, err := readFile(cs.BlkIO.IOPS.path)
			if err != nil {
				if os.IsNotExist(err) {
					delete(statsHolder.containers, id)
					continue
				}
				return nil, err
			}
			statsHolder.containers[id].BlkIO.IOPS.create(string(b))
		}
	}
	return statsHolder.containers, nil
}

func readFile(path string) (b []byte, err error) {
	b, err = ioutil.ReadFile(path)
	if err != nil {
		return
	}
	return
}

func walkFn(filePath string, info os.FileInfo, err error) error {
	if err != nil {
		return nil
	}

	matches := re.FindStringSubmatch(filePath)
	if len(matches) < 2 {
		return nil
	}
	id := matches[1]
	if info.IsDir() {
		if _, ok := statsHolder.containers[id]; !ok {
			statsHolder.containers[id] = &Cstats{}
		}
	} else {
		if _, ok := statsHolder.containers[id]; ok {
			baseName := path.Base(info.Name())
			switch baseName {
			case memFile:
				statsHolder.containers[id].Memory.path = filePath
			case cPUFile:
				statsHolder.containers[id].CPU.path = filePath
			case blkIOIOPSFile:
				statsHolder.containers[id].BlkIO.Bytes.path = filePath
			case blkIOBytesFile:
				statsHolder.containers[id].BlkIO.IOPS.path = filePath
			}
		}
	}

	return nil
}
