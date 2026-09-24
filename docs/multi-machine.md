# Multiple machines and Kubernetes clusters

One dashboard can show many machines and clusters. Each gets its own entry in the sidebar.

## Other VMs

One machine runs the dashboard (the hub). Every other machine runs the agent and streams to the hub over an outbound WebSocket, so it needs no open port.

The easy way: click **+ Add machine** in the sidebar. It gives you an install command with the hub URL and join token filled in.

By hand:

```bash
# On the hub:
infracanvas serve      # prints the join token and a ready-made join command

# On each other VM:
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh \
  | sudo bash -s -- --join <hub-url> --token <join-token>

# or, with the binary already there:
infracanvas start --backend <hub-url> --token <join-token>
```

Agents reconnect on their own and keep their identity across restarts.

If you'd rather have separate dashboards, install normally on each VM; each gets its own URL.

## Kubernetes clusters

A cluster doesn't need anything installed in it. Click **+** next to **Clusters** and drop a kubeconfig. The hub talks to the API server directly, the way `kubectl` does, so the API server has to be reachable from the machine running `infracanvas serve`.

- A kubeconfig with several contexts shows a picker; nothing is added until you choose.
- The dialog shows what the credential can do before you connect.
- An unreachable API server is reported right away and not saved.
- Adding the same context again updates the existing entry instead of creating a second one.
- If the credential can't list some resource kinds (nodes, secrets, persistent volumes), those are skipped and the rest of the cluster still shows.

On startup InfraCanvas also connects to the current context in `$KUBECONFIG` or `~/.kube/config`. Turn that off with `--discover-local-kubeconfig=false` or `INFRACANVAS_DISCOVER_LOCAL_KUBECONFIG=false`.

### Getting a kubeconfig

If `kubectl` works on this machine you already have one, at `$KUBECONFIG` or `~/.kube/config`. To share only one cluster from it:

```bash
kubectl config view --minify --flatten > this-cluster-only.yaml
```

`--flatten` inlines certificates the file references by path, so the copy is self-contained.

From a cloud provider:

```bash
aws eks update-kubeconfig --name <cluster> --region <region>          # EKS
gcloud container clusters get-credentials <cluster> --zone <zone>     # GKE
az aks get-credentials --resource-group <rg> --name <cluster>         # AKS
```

These kubeconfigs authenticate with an `exec:` plugin (`aws`, `gcloud`, `az`). That CLI has to be installed and logged in on the machine running `infracanvas serve`.
