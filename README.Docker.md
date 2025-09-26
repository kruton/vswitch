# Docker Usage Guide

This document describes how to use the vswitch-for-qemu application with Docker.

## Building the Docker Image

```bash
make docker-build
```

This creates a multi-stage Docker image:
- **Build stage**: Uses `golang:1.21-alpine` to compile the application
- **Runtime stage**: Uses minimal `alpine:latest` with the compiled binary
- **Final size**: ~12MB

## Running the Container

### Interactive Mode (Help/Testing)
```bash
make docker-run
# or
docker run --rm -it -p 9999:9999 -p 9998:9998 vswitch-for-qemu:latest
```

### Daemon Mode
```bash
make docker-run-daemon
# or
docker run -d --restart unless-stopped -p 9999:9999 -p 9998:9998 --name vswitch-daemon vswitch-for-qemu:latest vswitch -daemon -ports 9999,9998
```

### Custom Ports
```bash
docker run --rm -it -p 8080:8080 -p 8081:8081 vswitch-for-qemu:latest vswitch -ports 8080,8081
```

### With Statistics Server
```bash
docker run --rm -it -p 9999:9999 -p 9998:9998 -p 8080:8080 vswitch-for-qemu:latest vswitch -ports 9999,9998 -stats-port 8080
```

## Container Management

### Stop Daemon Container
```bash
make docker-stop
# or
docker stop vswitch-daemon && docker rm vswitch-daemon
```

### View Logs
```bash
docker logs vswitch-daemon
# or for live logs
docker logs -f vswitch-daemon
```

### Shell Access (Debugging)
```bash
# Only works in interactive terminal
docker run --rm -it --entrypoint /bin/sh vswitch-for-qemu:latest
```

### Container Status
```bash
docker ps | grep vswitch
```

## Docker Image Details

- **Base Image**: `alpine:latest`
- **Go Version**: 1.21
- **User**: Non-root user `vswitch` (UID/GID: 1001)
- **Working Directory**: `/var/run/vswitch` (for PID files)
- **Exposed Ports**: 9999, 9998 (default)
- **Binary Location**: `/usr/local/bin/vswitch`

## Network Configuration

The container exposes the virtual switch ports. QEMU VMs can connect to these ports:

```bash
# In QEMU VM configuration
-netdev socket,connect=localhost:9999,id=net0
-device virtio-net-pci,netdev=net0
```

## Cleanup

### Remove Images
```bash
make docker-clean
# or
docker rmi vswitch-for-qemu:1.0.0 vswitch-for-qemu:latest
```

### Remove All (including containers)
```bash
docker stop vswitch-daemon 2>/dev/null || true
docker rm vswitch-daemon 2>/dev/null || true
docker rmi vswitch-for-qemu:1.0.0 vswitch-for-qemu:latest 2>/dev/null || true
```

## Security Considerations

- Container runs as non-root user (`vswitch`)
- Only necessary packages installed in runtime image
- No shell utilities in runtime image (minimal attack surface)
- Use `--read-only` flag for additional security:
  ```bash
  docker run --rm -it --read-only -p 9999:9999 -p 9998:9998 vswitch-for-qemu:latest
  ```

## Troubleshooting

### Port Already in Use
```bash
# Check what's using the port
netstat -tlnp | grep 9999
# or
lsof -i :9999
```

### Container Won't Start
```bash
# Check container logs
docker logs vswitch-daemon

# Check if ports are available
docker run --rm vswitch-for-qemu:latest vswitch -ports 9999,9998
```

### Performance Issues
- Use `--cpus` to limit CPU usage
- Use `--memory` to limit memory usage
- Monitor with `docker stats vswitch-daemon`
