package gocstat

import (
	"strconv"
	"strings"
	"time"
)

const (
	blkIOFile      = "blkio.io_serviced"
	blkIOBytesFile = "blkio.io_service_bytes"
)

// Block device input/output statistics
type BlkIOStat struct {
	Bytes BlkServiced
	IOPS  BlkServiced
}

// Block device tallies
type BlkServiced struct {
	commonFields
	Devices []BlkDevice
}

type BlkDevice struct {
	// block device major number
	Major int
	// block device minor number
	Minor int
	// units read
	Read uint64
	// units written
	Write uint64
	// synchronous operation count
	Sync uint64
	// asynchronous operation count
	Async uint64
}

func (b *BlkServiced) create(content string) {
	b.Timestamp = time.Now()
	lines := strings.Split(content, "\n")
	lastDeviceStr := ""
	tmpContent := ""
	b.Devices = make([]BlkDevice, 0)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		deviceStr := fields[0]

		if deviceStr != lastDeviceStr {
			if lastDeviceStr != "" {
				bd := BlkDevice{}
				bd.create(tmpContent)
				b.Devices = append(b.Devices, bd)
			}
			tmpContent = ""
		} else {
			tmpContent += line
		}
	}
	if tmpContent != "" {
		bd := BlkDevice{}
		bd.create(tmpContent)
		b.Devices = append(b.Devices, bd)
	}
}

func (b *BlkDevice) create(content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		deviceStr := fields[0]
		op := fields[1]
		unit := fields[2]

		device := strings.Split(deviceStr, ":")
		b.Major, _ = strconv.Atoi(device[0])
		if len(device) == 2 {
			b.Major, _ = strconv.Atoi(device[0])
			b.Minor, _ = strconv.Atoi(device[1])
		}

		switch op {
		case "Read":
			b.Read, _ = strconv.ParseUint(unit, 10, 64)
		case "Write":
			b.Write, _ = strconv.ParseUint(unit, 10, 64)
		case "Sync":
			b.Sync, _ = strconv.ParseUint(unit, 10, 64)
		case "Async":
			b.Async, _ = strconv.ParseUint(unit, 10, 64)
		}
	}
}
