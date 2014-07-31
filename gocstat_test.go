package gocstat

import (
	//	"fmt"
	"testing"
)

func TestInit(t *testing.T) {

	BasePath = "sys/fs/cgroup"
	err := Init(nil)
	if err != nil {
		t.Errorf("Init error %s", err)
	}
}

func TestContainersLen(t *testing.T) {
	if len(h.containerStats) != 1 {
		t.Errorf("Expected 1 container, found %d", len(h.containerStats))
	}
}

func TestReadStats(t *testing.T) {
	stats, err := ReadStats()
	if err != nil {
		t.Error(err)
	}
	for _, stat := range stats {
		if stat.Memory.Cache == 0 {
			t.Errorf("Memory.Cache: expected non-zero value")
		}
		if stat.Memory.RSS == 0 {
			t.Errorf("Memory.RSS: expected non-zero value")
		}
		if stat.CPU.User == 0 {
			t.Errorf("CPU.User: expected non-zero value")
		}
		if stat.CPU.System == 0 {
			t.Errorf("CPU.System: expected non-zero value")
		}

		for _, dev := range stat.BlkIO.Bytes.Devices {
			if dev.Read == 0 || dev.Write == 0 ||
				dev.Sync == 0 || dev.Async == 0 {
				t.Errorf("BlkIO.Bytes.Device '%d:%d': expected non-zero value for all fields", dev.Major, dev.Minor)
			}
		}
		for _, dev := range stat.BlkIO.IOPS.Devices {
			if dev.Read == 0 || dev.Write == 0 ||
				dev.Sync == 0 || dev.Async == 0 {
				t.Errorf("BlkIO.IOPS.Device '%d:%d': expected non-zero value for all fields", dev.Major, dev.Minor)
			}
		}
	}
}
