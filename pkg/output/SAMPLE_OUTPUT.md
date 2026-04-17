# Sample Output Examples

This document shows examples of the different output formats produced by the formatters.

## Table Format

```
Infrastructure Snapshot - 2024-01-15T10:30:45Z
Host ID: host-1
Collection Duration: 2.5s
Scope: host, docker, kubernetes

=== HOST (1) ===
HOSTNAME      OS                    CPU     MEMORY  HEALTH   
------------  --------------------  ------  ------  -------  
test-host     Linux Ubuntu 22.04    45.5%   60.2%   healthy  

=== PROCESS (3) ===
PID    NAME           USER    CPU%   MEM%   TYPE      
-----  -------------  ------  -----  -----  --------  
1234   dockerd        root    5.2    2.1    docker    
5678   kubelet        root    8.5    3.4    kubelet   
9012   nginx          www     1.2    0.8    webserver 

=== SERVICE (2) ===
NAME      STATUS   ENABLED  CRITICAL  HEALTH   
--------  -------  -------  --------  -------  
docker    active   Yes      Yes       healthy  
kubelet   active   Yes      Yes       healthy  

=== CONTAINER (2) ===
NAME            IMAGE          STATE    CPU%   MEMORY   HEALTH   
--------------  -------------  -------  -----  -------  -------  
web-app         nginx:latest   running  10.5   100.0MB  healthy  
api-server      api:v1.2.3     running  25.3   256.0MB  healthy  

=== IMAGE (2) ===
REPOSITORY     TAG      SIZE     CREATED    
-------------  -------  -------  ---------  
nginx          latest   142.0MB  5d ago     
api            v1.2.3   512.0MB  2h ago     

=== POD (3) ===
NAMESPACE  NAME                    PHASE    NODE      RESTARTS  HEALTH   
---------  ----------------------  -------  --------  --------  -------  
default    web-app-7d8f9c-abc123   Running  node-1    0         healthy  
default    api-server-5f6g7h-def   Running  node-1    2         degraded 
kube-sys   coredns-8c9d0e-ghi456   Running  node-2    0         healthy  

=== DEPLOYMENT (2) ===
NAMESPACE  NAME        READY  UP-TO-DATE  AVAILABLE  HEALTH   
---------  ----------  -----  ----------  ---------  -------  
default    web-app     3/3    3           3          healthy  
default    api-server  2/3    2           2          degraded 

=== NODE (2) ===
NAME    STATUS  ROLES           VERSION  HEALTH   
------  ------  --------------  -------  -------  
node-1  Ready   control-plane   v1.28.0  healthy  
node-2  Ready   worker          v1.28.0  healthy  

=== K8S_SERVICE (2) ===
NAMESPACE  NAME        TYPE        CLUSTER-IP    ENDPOINTS  
---------  ----------  ----------  ------------  ---------  
default    web-app     ClusterIP   10.96.0.100   Yes        
default    api-server  NodePort    10.96.0.101   Yes        

=== RELATIONSHIPS (15) ===
RELATION TYPE  COUNT  
-------------  -----  
RUNS_ON        5      
USES           4      
TARGETS        3      
MOUNTS         2      
REFERENCES     1      
```

## JSON Format (Pretty-Printed)

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "host_id": "host-1",
  "entities": {
    "host-1": {
      "id": "host-1",
      "type": "host",
      "labels": {
        "env": "production"
      },
      "annotations": {},
      "health": "healthy",
      "timestamp": "2024-01-15T10:30:45Z",
      "hostname": "test-host",
      "fqdn": "test-host.example.com",
      "machine_id": "abc123def456",
      "os": "Linux",
      "os_version": "Ubuntu 22.04",
      "kernel_version": "5.15.0-91-generic",
      "architecture": "x86_64",
      "virtualization_type": "kvm",
      "cpu_model": "Intel Xeon E5-2680",
      "cpu_cores": 8,
      "cpu_usage_percent": 45.5,
      "memory_total_bytes": 16777216000,
      "memory_used_bytes": 10066329600,
      "memory_usage_percent": 60.2,
      "network_interfaces": [
        {
          "name": "eth0",
          "ip_addresses": ["192.168.1.100"],
          "mac_address": "00:11:22:33:44:55",
          "status": "up"
        }
      ],
      "listening_ports": [
        {
          "port": 22,
          "protocol": "tcp",
          "process_id": 1234,
          "process": "sshd"
        }
      ],
      "filesystems": [
        {
          "mount_point": "/",
          "device": "/dev/sda1",
          "fs_type": "ext4",
          "total_bytes": 107374182400,
          "used_bytes": 53687091200,
          "avail_bytes": 53687091200,
          "usage_percent": 50.0
        }
      ]
    },
    "container-1": {
      "id": "container-1",
      "type": "container",
      "health": "healthy",
      "timestamp": "2024-01-15T10:30:45Z",
      "container_id": "abc123def456",
      "name": "web-app",
      "image": "nginx:latest",
      "image_id": "sha256:abc123",
      "state": "running",
      "status": "Up 5 hours",
      "created": "2024-01-15T05:30:45Z",
      "started": "2024-01-15T05:30:46Z",
      "restart_count": 0,
      "cpu_percent": 10.5,
      "memory_usage": 104857600,
      "memory_limit": 536870912,
      "network_rx_bytes": 1048576,
      "network_tx_bytes": 2097152,
      "block_read_bytes": 4194304,
      "block_write_bytes": 8388608,
      "environment": {
        "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "NGINX_VERSION": "1.25.3"
      },
      "port_mappings": [
        {
          "host_ip": "0.0.0.0",
          "host_port": 8080,
          "container_port": 80,
          "protocol": "tcp"
        }
      ],
      "mounts": [],
      "network_mode": "bridge"
    }
  },
  "relations": [
    {
      "source_id": "container-1",
      "target_id": "host-1",
      "type": "RUNS_ON",
      "properties": {}
    },
    {
      "source_id": "container-1",
      "target_id": "image-1",
      "type": "USES",
      "properties": {}
    }
  ],
  "metadata": {
    "collection_duration": 2500000000,
    "scope": ["host", "docker", "kubernetes"],
    "errors": [],
    "permission_issues": []
  }
}
```

## YAML Format

```yaml
timestamp: 2024-01-15T10:30:45Z
host_id: host-1
entities:
  host-1:
    id: host-1
    type: host
    labels:
      env: production
    annotations: {}
    health: healthy
    timestamp: 2024-01-15T10:30:45Z
    hostname: test-host
    fqdn: test-host.example.com
    machine_id: abc123def456
    os: Linux
    os_version: Ubuntu 22.04
    kernel_version: 5.15.0-91-generic
    architecture: x86_64
    virtualization_type: kvm
    cpu_model: Intel Xeon E5-2680
    cpu_cores: 8
    cpu_usage_percent: 45.5
    memory_total_bytes: 16777216000
    memory_used_bytes: 10066329600
    memory_usage_percent: 60.2
    network_interfaces:
      - name: eth0
        ip_addresses:
          - 192.168.1.100
        mac_address: "00:11:22:33:44:55"
        status: up
    listening_ports:
      - port: 22
        protocol: tcp
        process_id: 1234
        process: sshd
    filesystems:
      - mount_point: /
        device: /dev/sda1
        fs_type: ext4
        total_bytes: 107374182400
        used_bytes: 53687091200
        avail_bytes: 53687091200
        usage_percent: 50.0
  container-1:
    id: container-1
    type: container
    health: healthy
    timestamp: 2024-01-15T10:30:45Z
    container_id: abc123def456
    name: web-app
    image: nginx:latest
    image_id: sha256:abc123
    state: running
    status: Up 5 hours
    created: 2024-01-15T05:30:45Z
    started: 2024-01-15T05:30:46Z
    restart_count: 0
    cpu_percent: 10.5
    memory_usage: 104857600
    memory_limit: 536870912
    network_rx_bytes: 1048576
    network_tx_bytes: 2097152
    block_read_bytes: 4194304
    block_write_bytes: 8388608
    environment:
      PATH: /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
      NGINX_VERSION: 1.25.3
    port_mappings:
      - host_ip: 0.0.0.0
        host_port: 8080
        container_port: 80
        protocol: tcp
    mounts: []
    network_mode: bridge
relations:
  - source_id: container-1
    target_id: host-1
    type: RUNS_ON
    properties: {}
  - source_id: container-1
    target_id: image-1
    type: USES
    properties: {}
metadata:
  collection_duration: 2500000000
  scope:
    - host
    - docker
    - kubernetes
  errors: []
  permission_issues: []
```

## Progress Indicators

### Spinner Progress
```
⠋ Discovering infrastructure...
⠙ Collecting host information...
⠹ Collecting Docker containers...
⠸ Collecting Kubernetes resources...
✓ Completed in 2.5s
```

### Multi-Stage Progress
```
⠋ [1/5] Discovering host information
⠙ [2/5] Discovering Docker containers
⠹ [3/5] Discovering Kubernetes resources
⠸ [4/5] Building relationships
⠼ [5/5] Calculating health status
✓ Completed in 3.2s
```

### Simple Progress
```
→ Starting discovery...
✓ Host information collected
✓ Docker containers discovered
⚠ Kubernetes API not accessible, skipping
✓ Relationships built
✓ Health status calculated
```

## Color Coding

In terminal output, health statuses are color-coded:

- **Green** (healthy): ✓ healthy, running, active, ready
- **Yellow** (degraded): ⚠ degraded, pending, inactive
- **Red** (unhealthy): ✗ unhealthy, failed, exited, not ready

Example:
```
HEALTH
-------
healthy   (displayed in green)
degraded  (displayed in yellow)
unhealthy (displayed in red)
```
