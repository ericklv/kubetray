# K8s Context Switcher & Sync

Bash script to automatically scan all Google Cloud Platform (GCP) projects across your organization or account, discover **Google Kubernetes Engine (GKE)** clusters, and register their credentials into your local `kubeconfig` while preventing duplicate entries.

---

## 🚀 Features

- **Interactive Authentication**: Automatically detects whether an active `gcloud` session exists. If not, it prompts in the terminal to open your browser and authenticate (`gcloud auth login`).
- **System Projects Filter**: Automatically excludes all projects matching the `sys-*` prefix.
- **Duplicate Prevention**: Checks if the target context (`gke_<PROJECT>_<LOCATION>_<CLUSTER>`) already exists in `kubeconfig` before fetching credentials, avoiding redundant operations.
- **Resilient Error Handling**: Safely continues if a project has the Container API disabled or lacks permissions, without interrupting the overall execution.
- **Dry-Run Simulation**: Preview all clusters that would be added without modifying your `kubeconfig`.
- **Statistical Summary**: Displays a clean summary with total projects scanned, clusters found, contexts added, and duplicates skipped.

---

## 📋 Prerequisites

- **[Google Cloud SDK (`gcloud`)](https://cloud.google.com/sdk/docs/install)**
- **[`kubectl`](https://kubernetes.io/docs/tasks/tools/)**
- **[`gke-gcloud-auth-plugin`](https://cloud.google.com/blog/products/containers-kubernetes/introducing-gke-gcloud-auth-plugin)** (required by Kubernetes v1.26+ for GKE authentication)
  ```bash
  gcloud components install gke-gcloud-auth-plugin
  ```
- **Bash 4.0+**

---

## 🛠️ Installation & Usage

1. Grant execute permissions to the script:
   ```bash
   chmod +x sync-gke.sh
   ```

2. Run the script:
   ```bash
   ./sync-gke.sh
   ```

---

## ⚙️ Options & Flags

| Flag | Description |
| :--- | :--- |
| `-d`, `--dry-run` | Shows which clusters would be added without modifying `kubeconfig`. |
| `-f`, `--force` | Forces fetching credentials even if the context already exists. |
| `-i`, `--internal-ip` | Registers clusters using their private endpoint (`--internal-ip`). |
| `-v`, `--verbose` | Shows detailed debug output during execution. |
| `-h`, `--help` | Displays the help menu. |

### Examples

**Dry-run simulation:**
```bash
./sync-gke.sh --dry-run
```

**Standard synchronization:**
```bash
./sync-gke.sh
```

**Force update existing credentials:**
```bash
./sync-gke.sh --force
```

---

## 🖱️ Tray app: `k8s-context-switcher`

Tray icon to switch the current kubeconfig context, built the same way
as `browser-switcher`: no GTK, the icon and menu talk directly to the
freedesktop StatusNotifierItem and com.canonical.dbusmenu D-Bus
interfaces, so the only runtime dependencies are D-Bus and `kubectl`.

- The menu lists every context from `kubectl config get-contexts`,
  with a radio mark on the current one. GKE contexts
  (`gke_<project>_<location>_<cluster>`) are grouped under their
  project and shown as `cluster · location`; anything else goes under
  "Other".
- Clicking one runs `kubectl config use-context <name>`.
- The kubeconfig files (`$KUBECONFIG`, else `~/.kube/config`) are
  polled every 2s, so a `kubectl config use-context` from a terminal,
  or new clusters added by `sync-gke.sh`, show up without restarting.
- The tooltip shows the current cluster and project. The icon turns
  red when the current context name contains `prd`, `prod` or
  `production` as a separate word.
- If the current context's credential plugin needs a cloud login that
  isn't active, a warning entry with the fix command appears below the
  contexts (GCP: `gcloud auth print-access-token`; AWS: `aws sts
  get-caller-identity` with the plugin's profile; Azure: `az account
  get-access-token`, only for kubelogin's `azurecli` mode). Rechecked
  every minute.
- "Launch at login" toggles `~/.config/autostart/k8s-context-switcher.desktop`.

`kubectl` is looked up in `PATH`, then in `~/google-cloud-sdk/bin`,
`~/.local/bin`, `/usr/local/bin` and `/usr/bin` (autostart sessions
often don't have your shell's `PATH`). Set `$KUBECTL` to override.

Requires a StatusNotifierHost to display the icon, e.g. waybar's
`tray` module, or `snixembed` if your bar doesn't support
StatusNotifierItem.

```bash
make build                # builds ./k8s-context-switcher
sudo make install         # installs to /usr/local/bin (PREFIX=/usr to change)
sudo make uninstall
```

---

## 📄 License

MIT
