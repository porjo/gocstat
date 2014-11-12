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

package gocstat

import (
	//	"fmt"
	"testing"
)

func TestInit(t *testing.T) {
	BasePath = "testdata/cgroup"
	err := Init(nil)
	if err != nil {
		t.Errorf("Init error %s", err)
	}
}

func TestContainersLen(t *testing.T) {
	if len(statsHolder.containers) != 1 {
		t.Errorf("Expected 1 container, found %d", len(statsHolder.containers))
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
