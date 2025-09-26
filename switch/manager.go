package vswitch

import (
	"fmt"
	"log"
	"sync"
)

// SwitchManager manages multiple isolated virtual switches (VLANs)
type SwitchManager struct {
	switches map[int]*VirtualSwitch // port -> switch mapping
	mutex    sync.RWMutex
}

// NewSwitchManager creates a new switch manager
func NewSwitchManager() *SwitchManager {
	return &SwitchManager{
		switches: make(map[int]*VirtualSwitch),
	}
}

// AddVLAN creates a new isolated VLAN on the specified port
func (sm *SwitchManager) AddVLAN(port int) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.switches[port]; exists {
		return fmt.Errorf("VLAN already exists on port %d", port)
	}

	// Create a single-port virtual switch for this VLAN
	vs := NewVirtualSwitch([]int{port})
	sm.switches[port] = vs

	log.Printf("Created VLAN on port %d", port)
	return nil
}

// RemoveVLAN removes a VLAN and stops its switch
func (sm *SwitchManager) RemoveVLAN(port int) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	vs, exists := sm.switches[port]
	if !exists {
		return fmt.Errorf("VLAN does not exist on port %d", port)
	}

	vs.Stop()
	delete(sm.switches, port)

	log.Printf("Removed VLAN on port %d", port)
	return nil
}

// StartAll starts all VLANs
func (sm *SwitchManager) StartAll() error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for port, vs := range sm.switches {
		if err := vs.Start(); err != nil {
			return fmt.Errorf("failed to start VLAN on port %d: %v", port, err)
		}
		log.Printf("Started VLAN on port %d", port)
	}

	return nil
}

// StopAll stops all VLANs
func (sm *SwitchManager) StopAll() {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for port, vs := range sm.switches {
		vs.Stop()
		log.Printf("Stopped VLAN on port %d", port)
	}
}

// GetVLANs returns a list of active VLAN ports
func (sm *SwitchManager) GetVLANs() []int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	ports := make([]int, 0, len(sm.switches))
	for port := range sm.switches {
		ports = append(ports, port)
	}

	return ports
}

// GetStats returns aggregated statistics from all VLANs
func (sm *SwitchManager) GetStats() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	totalFrames := uint64(0)
	totalBroadcast := uint64(0)
	totalUnicast := uint64(0)
	totalDropped := uint64(0)
	totalConnections := 0
	totalMACEntries := 0

	vlanStats := make(map[string]interface{})

	for port, vs := range sm.switches {
		stats := vs.GetStats()

		totalFrames += stats["total_frames"].(uint64)
		totalBroadcast += stats["broadcast_frames"].(uint64)
		totalUnicast += stats["unicast_frames"].(uint64)
		totalDropped += stats["dropped_frames"].(uint64)
		totalConnections += stats["connections"].(int)
		totalMACEntries += stats["mac_entries"].(int)

		vlanStats[fmt.Sprintf("vlan_%d", port)] = stats
	}

	return map[string]interface{}{
		"total_frames":     totalFrames,
		"broadcast_frames": totalBroadcast,
		"unicast_frames":   totalUnicast,
		"dropped_frames":   totalDropped,
		"total_connections": totalConnections,
		"total_mac_entries": totalMACEntries,
		"vlans":            vlanStats,
		"vlan_count":       len(sm.switches),
	}
}
