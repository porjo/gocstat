package gocstat

import (
	"strconv"
	"strings"
	"time"
)

const (
	blkIOIOPSFile  = "blkio.io_serviced"
	blkIOBytesFile = "blkio.io_service_bytes"
)

// Block device input/output statistics
type BlkIOStat struct {
	Bytes BlkServiced
	IOPS  BlkServiced
}

// Block device tallies
type BlkServiced struct {
	CommonFields
	Devices []BlkDevice
}

type BlkDevice struct {
	// block device major number
	Major uint64
	// block device minor number
	Minor uint64
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
	tmpContent := make([]string, 0)
	b.Devices = make([]BlkDevice, 0)
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		deviceStr := fields[0]

		if deviceStr != lastDeviceStr && i > 0 {
			bd := BlkDevice{}
			bd.create(tmpContent)
			b.Devices = append(b.Devices, bd)
			tmpContent = make([]string, 0)
		}
		tmpContent = append(tmpContent, line)
		lastDeviceStr = deviceStr
	}
	if len(tmpContent) != 0 {
		bd := &BlkDevice{}
		bd.create(tmpContent)
		b.Devices = append(b.Devices, *bd)
	}
}

func (b *BlkDevice) create(lines []string) {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		deviceStr := fields[0]
		op := fields[1]
		unit := fields[2]

		device := strings.Split(deviceStr, ":")
		if len(device) > 1 {
			b.Major, _ = strconv.ParseUint(device[0], 10, 64)
			b.Minor, _ = strconv.ParseUint(device[1], 10, 64)
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
