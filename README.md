# Virtual Switch for QEMU VMs

A high-performance virtual Ethernet switch that accepts socket connections from QEMU VMs and provides MAC learning and frame forwarding capabilities with isolated VLANs.

## Features

- **Isolated VLANs**: Each port creates a separate isolated virtual LAN
- **MAC Learning**: Automatically learns VM MAC addresses from incoming frames
- **Efficient Forwarding**: Direct unicast forwarding based on learned MAC table
- **Broadcast Handling**: Intelligent flooding of broadcast frames within each VLAN
- **Connection Management**: Proper cleanup when VMs disconnect
- **Daemon Mode**: Can run in background with PID file management
- **Concurrent Safe**: Thread-safe operations for multiple simultaneous VM connections
- **Performance Optimized**: Efficient frame processing with minimal overhead

## Usage

```bash
# Start with multiple isolated VLANs (foreground)
./vswitch -ports 9999,9998

# Start as daemon in background
./vswitch -daemon -ports 9999,9998

# Check daemon status
./vswitch -status

# Stop daemon
./vswitch -stop

# Custom configuration
./vswitch -ports 8080,8081 -log-level debug
```

## Architecture

The virtual switch creates isolated VLANs where:
- Each port represents a separate virtual network segment
- VMs connecting to the same port can communicate with each other
- VMs on different ports are completely isolated
- MAC learning and forwarding occurs independently within each VLAN

### Network Isolation

As an example, you may have this mapping:

- **Port 9999**: VLAN 1 (e.g., internal network)
- **Port 9998**: VLAN 2 (e.g., external network)

## Integration with QEMU

Configure QEMU VMs to connect to specific VLANs:

```bash
# VM connecting to VLAN 1 (port 9999)
-netdev socket,id=net0,connect=:9999
-device virtio-net-pci,netdev=net0

# VM connecting to VLAN 2 (port 9998)
-netdev socket,id=net0,connect=:9998
-device virtio-net-pci,netdev=net0
```

## Daemon Management

The virtual switch supports daemon mode for production deployments:

```bash
# Start daemon with custom PID and log files
./vswitch -daemon -pid-file /var/run/vswitch.pid -log-file /var/log/vswitch.log

# Check if daemon is running
./vswitch -status -pid-file /var/run/vswitch.pid

# Stop daemon
./vswitch -stop -pid-file /var/run/vswitch.pid
```

## Replacing QEMU Hubport Networking

This virtual switch replaces complex QEMU hubport configurations while providing proper Ethernet switching semantics and better network isolation. Instead of managing multiple hubport configurations, simply:

1. Start the virtual switch with desired ports
2. Configure QEMU VMs to connect to appropriate ports
3. Enjoy automatic MAC learning and proper frame forwarding

## Building

```bash
# Build the application
make build

# Install to /usr/local/bin
make install

# Clean build artifacts
make clean
```
