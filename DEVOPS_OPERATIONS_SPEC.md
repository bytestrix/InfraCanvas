# DevOps Operations Platform - Comprehensive Specification

## Problem Statement

Current workflow for updating frontend images across 150 VMs:
- **69 VMs**: Make release local → Get image tag → kubectl get pods → docker push
- **150 VMs**: Get cs pods → k get deploy a → k get deploy h → name container → change image

This is extremely time-consuming, error-prone, and doesn't scale.

## Solution: Unified DevOps Operations Platform

Transform InfraCanvas into a comprehensive DevOps control plane that allows users to perform all operations from a single interface across multiple VMs simultaneously.

---

## Core Features

### 1. Bulk Operations (Multi-VM Selection)
- Select multiple VMs from the canvas
- Execute operations on all selected VMs simultaneously
- Real-time progress tracking for each VM
- Rollback capability if operations fail

### 2. Docker Operations
- **Build & Push**
  - Build Docker image from Dockerfile
  - Tag image with custom or auto-generated tags
  - Push to registry (Docker Hub, ECR, GCR, ACR, private registry)
  - Multi-stage build support
  
- **Image Management**
  - Pull images from registry
  - Tag/retag images
  - Remove unused images (cleanup)
  - List image history and layers
  - Scan images for vulnerabilities
  
- **Container Operations**
  - Start/Stop/Restart containers
  - Remove containers
  - View container logs (live streaming)
  - Execute commands in containers
  - Inspect container details
  - Update container resources (CPU, memory limits)

### 3. Kubernetes Operations
- **Deployment Management**
  - Update deployment image (the key use case!)
  - Scale deployments up/down
  - Rollout restart
  - Rollback to previous version
  - Pause/Resume rollouts
  - View rollout status and history
  
- **Pod Operations**
  - Restart pods (delete to trigger recreation)
  - View pod logs (live streaming, multi-container support)
  - Execute commands in pods
  - Port-forward to pods
  - Copy files to/from pods
  - Describe pod details
  
- **Service Operations**
  - Update service selectors
  - Change service type (ClusterIP, NodePort, LoadBalancer)
  - View service endpoints
  
- **ConfigMap & Secret Management**
  - Create/Update/Delete ConfigMaps
  - Create/Update/Delete Secrets
  - View ConfigMap/Secret data (with redaction for secrets)
  
- **Resource Management**
  - Apply YAML manifests
  - Delete resources by name or label selector
  - Get resource status
  - Patch resources (strategic merge, JSON patch)

### 4. CI/CD Pipeline Operations
- **Release Workflow** (solves your exact problem!)
  - One-click release across multiple VMs:
    1. Build image on designated build VM
    2. Push to registry
    3. Update deployments on all target VMs
    4. Monitor rollout status
    5. Auto-rollback on failure
  
- **Deployment Strategies**
  - Rolling update (default)
  - Blue-green deployment
  - Canary deployment
  - A/B testing support
  
- **Pipeline Templates**
  - Save common workflows as templates
  - Parameterized templates (image tag, replicas, etc.)
  - Schedule recurring operations

### 5. Monitoring & Observability
- **Real-time Metrics**
  - Container CPU/Memory usage
  - Pod resource consumption
  - Deployment health status
  - Node resource availability
  
- **Log Aggregation**
  - View logs from multiple pods simultaneously
  - Filter logs by time range, severity
  - Search across logs
  - Export logs
  
- **Event Tracking**
  - Kubernetes events
  - Docker events
  - Operation audit log

### 6. Batch Operations
- **Image Update Workflow**
  - Select deployments across multiple VMs
  - Update all to new image version
  - Staggered rollout (update 10% at a time)
  - Health checks between batches
  
- **Configuration Updates**
  - Update ConfigMaps/Secrets across VMs
  - Trigger pod restarts after config changes
  
- **Cleanup Operations**
  - Remove unused images across all VMs
  - Delete completed pods
  - Prune Docker system

### 7. Safety & Validation
- **Pre-flight Checks**
  - Validate image exists in registry
  - Check resource availability
  - Verify RBAC permissions
  
- **Dry-run Mode**
  - Preview changes before applying
  - Show what would be affected
  
- **Approval Workflows**
  - Require confirmation for destructive operations
  - Multi-stage approval for production
  
- **Rollback Protection**
  - Automatic rollback on failure
  - Manual rollback to any previous version
  - Keep deployment history

### 8. Advanced Features
- **Scheduled Operations**
  - Cron-based scheduling
  - Maintenance windows
  - Auto-scaling schedules
  
- **Webhooks & Notifications**
  - Slack/Teams/Discord notifications
  - Webhook triggers for external systems
  - Email alerts on failures
  
- **RBAC & Access Control**
  - User roles (viewer, operator, admin)
  - VM-level permissions
  - Operation-level permissions
  - Audit logging

---

## Implementation Architecture

### Backend Components

#### 1. Action System Enhancement
```
pkg/actions/
├── types.go              # Extended action types
├── executor.go           # Main executor with bulk support
├── docker_advanced.go    # Build, push, image management
├── kubernetes_advanced.go # Deployment updates, rollouts
├── batch.go              # Bulk operation orchestrator
├── pipeline.go           # CI/CD pipeline executor
├── validation.go         # Pre-flight checks
└── rollback.go           # Rollback manager
```

#### 2. New Action Types
```go
// Docker Build & Registry
ActionDockerBuild
ActionDockerPush
ActionDockerPull
ActionDockerTag
ActionDockerImageRemove
ActionDockerImagePrune

// Kubernetes Deployments
ActionK8sUpdateImage       // ⭐ Your main use case
ActionK8sRolloutRestart
ActionK8sRolloutUndo
ActionK8sRolloutPause
ActionK8sRolloutResume
ActionK8sRolloutStatus

// Kubernetes Resources
ActionK8sApplyManifest
ActionK8sDeleteResource
ActionK8sPatchResource
ActionK8sGetResource

// ConfigMaps & Secrets
ActionK8sCreateConfigMap
ActionK8sUpdateConfigMap
ActionK8sCreateSecret
ActionK8sUpdateSecret

// Logs & Exec
ActionK8sGetLogs
ActionK8sExec
ActionDockerExec
ActionDockerLogs

// Batch Operations
ActionBatchUpdateImage     // ⭐ Update image across multiple VMs
ActionBatchScale
ActionBatchRestart
ActionBatchCleanup

// Pipeline Operations
ActionPipelineRelease      // ⭐ Full release workflow
ActionPipelineRollback
```

#### 3. WebSocket Protocol Extension
```typescript
// New message types
MsgActionRequest          // Browser → Server → Agent
MsgActionProgress         // Agent → Server → Browser
MsgActionResult           // Agent → Server → Browser
MsgBatchActionRequest     // For multi-VM operations
MsgBatchActionProgress    // Progress for each VM
```

### Frontend Components

#### 1. Operations Panel
```
frontend/components/operations/
├── OperationsPanel.tsx        # Main operations UI
├── ActionSelector.tsx         # Choose operation type
├── TargetSelector.tsx         # Select VMs and resources
├── ParameterForm.tsx          # Operation parameters
├── ProgressTracker.tsx        # Real-time progress
├── ResultsView.tsx            # Operation results
└── HistoryView.tsx            # Past operations
```

#### 2. Quick Actions
```
frontend/components/quickactions/
├── ImageUpdateWizard.tsx      # ⭐ Guided image update
├── DeploymentScaler.tsx       # Quick scale UI
├── LogViewer.tsx              # Multi-pod log viewer
├── RollbackManager.tsx        # Rollback interface
└── PipelineRunner.tsx         # Pipeline execution UI
```

#### 3. Bulk Operations UI
```
frontend/components/bulk/
├── VMSelector.tsx             # Multi-select VMs
├── ResourceSelector.tsx       # Select resources across VMs
├── BulkActionForm.tsx         # Configure bulk operation
├── ProgressMatrix.tsx         # Show progress for each VM
└── ResultsSummary.tsx         # Aggregate results
```

---

## User Workflows

### Workflow 1: Update Frontend Image Across 150 VMs (Your Main Use Case)

**Current Process**: 
- 69 VMs: Manual build, tag, push, kubectl commands
- 150 VMs: Manual kubectl get, edit, apply

**New Process**:
1. Click "Update Image" button
2. Select VMs (or use tags: "frontend-vms")
3. Enter:
   - Deployment name: `frontend`
   - New image: `myregistry/frontend:v2.1.0`
   - Or: Build new image from Dockerfile
4. Click "Execute"
5. Watch real-time progress across all 150 VMs
6. Auto-rollback if any VM fails health checks

**Time**: From hours to 2 minutes

### Workflow 2: Build and Deploy New Release

1. Select "Release Pipeline" template
2. Configure:
   - Build VM: `build-server-01`
   - Dockerfile path: `/app/Dockerfile`
   - Image tag: `v2.1.0` (or auto-generate)
   - Target VMs: Select 150 VMs
   - Deployment name: `frontend`
3. Pipeline executes:
   - Build image on build-server-01
   - Push to registry
   - Update deployments on all 150 VMs
   - Monitor rollout
   - Report success/failure
4. One-click rollback if needed

### Workflow 3: Scale Deployments During Traffic Spike

1. Select multiple VMs experiencing high load
2. Choose "Scale Deployment"
3. Select deployment: `api-server`
4. Set replicas: `10` (from current `3`)
5. Execute across all selected VMs
6. Monitor scaling progress

### Workflow 4: View Logs from Multiple Pods

1. Select VMs
2. Choose "View Logs"
3. Select pods by label: `app=frontend`
4. View aggregated logs in real-time
5. Filter by severity, time range
6. Search across all logs

### Workflow 5: Cleanup Unused Images

1. Select all VMs
2. Choose "Cleanup Images"
3. Configure:
   - Remove dangling images
   - Remove images older than 30 days
   - Keep last 5 versions
4. Dry-run to preview
5. Execute cleanup
6. View space reclaimed per VM

---

## API Design

### Action Request Format
```json
{
  "action_id": "uuid",
  "type": "k8s_update_image",
  "targets": [
    {
      "vm_code": "ABC123",
      "namespace": "default",
      "deployment": "frontend"
    }
  ],
  "parameters": {
    "image": "myregistry/frontend:v2.1.0",
    "container": "app"
  },
  "options": {
    "dry_run": false,
    "auto_rollback": true,
    "health_check_timeout": 300
  }
}
```

### Progress Update Format
```json
{
  "action_id": "uuid",
  "vm_code": "ABC123",
  "status": "in_progress",
  "progress": 45,
  "message": "Waiting for rollout to complete (2/5 pods updated)",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Result Format
```json
{
  "action_id": "uuid",
  "vm_code": "ABC123",
  "status": "success",
  "message": "Deployment updated successfully",
  "details": {
    "old_image": "myregistry/frontend:v2.0.0",
    "new_image": "myregistry/frontend:v2.1.0",
    "pods_updated": 5,
    "duration_seconds": 45
  },
  "timestamp": "2024-01-15T10:31:00Z"
}
```

---

## Security Considerations

1. **Authentication**: Token-based auth for agents
2. **Authorization**: RBAC for operations
3. **Audit Logging**: All operations logged with user, timestamp, parameters
4. **Secrets Management**: Never log or display secrets in plain text
5. **Rate Limiting**: Prevent abuse of bulk operations
6. **Validation**: Validate all inputs before execution
7. **Isolation**: Operations on one VM don't affect others

---

## Performance Considerations

1. **Parallel Execution**: Execute on multiple VMs concurrently (configurable parallelism)
2. **Streaming Results**: Stream progress updates, don't wait for all to complete
3. **Timeout Handling**: Configurable timeouts per operation
4. **Resource Limits**: Limit concurrent operations per agent
5. **Caching**: Cache frequently accessed data (image lists, deployment status)

---

## Monitoring & Observability

1. **Operation Metrics**:
   - Success/failure rates
   - Average execution time
   - Most common operations
   
2. **System Health**:
   - Agent connectivity status
   - Operation queue depth
   - Resource usage per agent
   
3. **Alerts**:
   - Failed operations
   - Agent disconnections
   - Resource exhaustion

---

## Future Enhancements

1. **GitOps Integration**: Sync with Git repositories
2. **Terraform Integration**: Manage infrastructure as code
3. **Helm Support**: Deploy Helm charts
4. **Service Mesh**: Istio/Linkerd operations
5. **Cost Optimization**: Identify and remove unused resources
6. **AI-Powered Suggestions**: Recommend optimizations
7. **Multi-Cloud**: Support AWS ECS, Azure Container Instances
8. **Compliance**: Policy enforcement, security scanning

---

## Implementation Priority

### Phase 1 (MVP - Solves Your Immediate Problem)
1. ✅ Kubernetes image update action
2. ✅ Bulk operation support (multi-VM)
3. ✅ Progress tracking
4. ✅ Basic rollback
5. ✅ Frontend UI for image updates

### Phase 2 (Enhanced Operations)
1. Docker build & push
2. Full pipeline support
3. Log viewing
4. Deployment scaling
5. ConfigMap/Secret management

### Phase 3 (Advanced Features)
1. Scheduled operations
2. Approval workflows
3. Advanced rollback strategies
4. Webhooks & notifications
5. RBAC & access control

### Phase 4 (Enterprise Features)
1. GitOps integration
2. Multi-cloud support
3. AI-powered insights
4. Cost optimization
5. Compliance & security scanning

---

## Success Metrics

1. **Time Savings**: Reduce deployment time from hours to minutes
2. **Error Reduction**: Eliminate manual errors
3. **Scalability**: Handle 1000+ VMs
4. **Reliability**: 99.9% operation success rate
5. **User Satisfaction**: Positive feedback from DevOps team

---

## Next Steps

1. Review and approve this specification
2. Implement Phase 1 (MVP)
3. Test with small subset of VMs
4. Roll out to production
5. Gather feedback and iterate
