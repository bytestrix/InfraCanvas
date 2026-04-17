# QuickStart

## 1. Start Platform (Local)

```bash
./start.sh
```

This starts:
- Backend server on :8080
- Frontend on :3000

## 2. Connect Agent (on VM)

SSH to your VM:
```bash
ssh rishi@40.81.240.69
cd /path/to/InfraCanvas
./bin/infracanvas start --backend-url ws://YOUR_LOCAL_IP:8080
```

Copy the PAIR CODE shown.

## 3. Open Browser

1. Go to http://localhost:3000
2. Click "Connect VM"
3. Enter the pair code
4. View infrastructure!

## 4. Use DevOps Operations

1. Click on VM card to open canvas
2. Click "Operations" button (top bar)
3. Choose "Update Image"
4. Fill in:
   - Namespace: `default`
   - Deployment: `frontend`
   - New Image: `myregistry/frontend:v2.1.0`
5. Click "Update Image on 1 VM"
6. Watch real-time progress!

## Multiple VMs

To update across 150 VMs:
1. Connect all 150 VMs (each gets a pair code)
2. In operations panel, it will show all connected VMs
3. Execute once, updates all!

---

## What's Implemented

✅ Kubernetes image updates
✅ Batch operations (multi-VM)
✅ Real-time progress tracking
✅ Auto-rollback on failure
✅ Deployment scaling
✅ Action history

## Architecture

```
Browser (:3000) ←→ Server (:8080) ←→ Agents (on VMs)
```

Each agent discovers its VM's infrastructure and sends to server.
Server relays to browser for visualization.
Operations flow: Browser → Server → Agent → Execute → Report back
