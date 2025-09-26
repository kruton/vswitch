package vswitch

import (
	"testing"
)

func TestNewSwitchManager(t *testing.T) {
	sm := NewSwitchManager()

	if sm.switches == nil {
		t.Errorf("Expected switches map to be initialized")
	}

	if len(sm.switches) != 0 {
		t.Errorf("Expected empty switches map, got %d entries", len(sm.switches))
	}
}

func TestSwitchManagerAddVLAN(t *testing.T) {
	sm := NewSwitchManager()

	// Add first VLAN
	err := sm.AddVLAN(8080)
	if err != nil {
		t.Errorf("Unexpected error adding VLAN: %v", err)
	}

	// Check that VLAN was added
	vlans := sm.GetVLANs()
	if len(vlans) != 1 {
		t.Errorf("Expected 1 VLAN, got %d", len(vlans))
	}

	if vlans[0] != 8080 {
		t.Errorf("Expected VLAN port 8080, got %d", vlans[0])
	}

	// Try to add duplicate VLAN
	err = sm.AddVLAN(8080)
	if err == nil {
		t.Errorf("Expected error when adding duplicate VLAN")
	}

	if err.Error() != "VLAN already exists on port 8080" {
		t.Errorf("Expected duplicate VLAN error, got: %v", err)
	}
}

func TestSwitchManagerRemoveVLAN(t *testing.T) {
	sm := NewSwitchManager()

	// Try to remove non-existent VLAN
	err := sm.RemoveVLAN(8080)
	if err == nil {
		t.Errorf("Expected error when removing non-existent VLAN")
	}

	if err.Error() != "VLAN does not exist on port 8080" {
		t.Errorf("Expected non-existent VLAN error, got: %v", err)
	}

	// Add and then remove VLAN
	sm.AddVLAN(8080)
	err = sm.RemoveVLAN(8080)
	if err != nil {
		t.Errorf("Unexpected error removing VLAN: %v", err)
	}

	// Check that VLAN was removed
	vlans := sm.GetVLANs()
	if len(vlans) != 0 {
		t.Errorf("Expected 0 VLANs after removal, got %d", len(vlans))
	}
}

func TestSwitchManagerGetVLANs(t *testing.T) {
	sm := NewSwitchManager()

	// Empty manager
	vlans := sm.GetVLANs()
	if len(vlans) != 0 {
		t.Errorf("Expected 0 VLANs for empty manager, got %d", len(vlans))
	}

	// Add multiple VLANs
	ports := []int{8080, 8081, 8082}
	for _, port := range ports {
		sm.AddVLAN(port)
	}

	vlans = sm.GetVLANs()
	if len(vlans) != len(ports) {
		t.Errorf("Expected %d VLANs, got %d", len(ports), len(vlans))
	}

	// Check that all ports are present (order doesn't matter)
	portMap := make(map[int]bool)
	for _, port := range vlans {
		portMap[port] = true
	}

	for _, expectedPort := range ports {
		if !portMap[expectedPort] {
			t.Errorf("Expected port %d to be in VLANs list", expectedPort)
		}
	}
}

func TestSwitchManagerGetStats(t *testing.T) {
	sm := NewSwitchManager()

	// Empty manager stats
	stats := sm.GetStats()

	expectedFields := []string{
		"total_frames", "broadcast_frames", "unicast_frames", "dropped_frames",
		"total_connections", "total_mac_entries", "vlans", "vlan_count",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected stats field '%s' to exist", field)
		}
	}

	// Check initial values
	if stats["vlan_count"] != 0 {
		t.Errorf("Expected vlan_count to be 0, got %v", stats["vlan_count"])
	}

	if stats["total_frames"] != uint64(0) {
		t.Errorf("Expected total_frames to be 0, got %v", stats["total_frames"])
	}

	// Add VLANs and check stats
	sm.AddVLAN(8080)
	sm.AddVLAN(8081)

	stats = sm.GetStats()
	if stats["vlan_count"] != 2 {
		t.Errorf("Expected vlan_count to be 2, got %v", stats["vlan_count"])
	}

	// Check vlans sub-stats
	vlansStats, ok := stats["vlans"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected vlans to be a map")
	} else {
		if _, exists := vlansStats["vlan_8080"]; !exists {
			t.Errorf("Expected vlan_8080 stats to exist")
		}
		if _, exists := vlansStats["vlan_8081"]; !exists {
			t.Errorf("Expected vlan_8081 stats to exist")
		}
	}
}

func TestSwitchManagerStartAll(t *testing.T) {
	sm := NewSwitchManager()

	// Add some VLANs first
	sm.AddVLAN(8080)
	sm.AddVLAN(8081)

	// Start all VLANs - this will fail in test environment since we can't bind to ports
	// but we can test that it doesn't panic and handles errors gracefully
	err := sm.StartAll()
	if err == nil {
		// This is unexpected in test environment, but not necessarily wrong
		// The function should handle the case where ports are available
	}

	// VLANs should still exist in the manager after StartAll attempt
	vlans := sm.GetVLANs()
	if len(vlans) != 2 {
		t.Errorf("Expected 2 VLANs after StartAll, got %d", len(vlans))
	}
}

func TestSwitchManagerStopAll(t *testing.T) {
	sm := NewSwitchManager()

	// Add some VLANs
	sm.AddVLAN(8080)
	sm.AddVLAN(8081)

	// StopAll should not panic
	sm.StopAll()

	// VLANs should still exist in the manager (StopAll doesn't remove them)
	vlans := sm.GetVLANs()
	if len(vlans) != 2 {
		t.Errorf("Expected 2 VLANs after StopAll, got %d", len(vlans))
	}
}
