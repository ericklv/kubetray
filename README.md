# KubeTray

Tray icon to switch the current Kubernetes context from your desktop bar.
No GTK: the icon and menu talk directly to the freedesktop
StatusNotifierItem and com.canonical.dbusmenu D-Bus interfaces, so the
only runtime dependencies are D-Bus and `kubectl`.

---

## Features

- **Context menu**: lists every context from `kubectl config get-contexts`,
  with a radio mark on the current one. GKE contexts
  (`gke_<project>_<location>_<cluster>`) are grouped under their project
  and shown as `cluster · location`; anything else goes under "Other".
- **One-click switch**: clicking a context runs
  `kubectl config use-context <name>`.
- **Live updates**: the kubeconfig files (`$KUBECONFIG`, else
  `~/.kube/config`) are polled every 2s, so a `kubectl config use-context`
  from a terminal, or newly added clusters, show up without restarting.
- **Production highlight**: the tooltip shows the current cluster and
  project, and the icon turns red when the current context name contains
  `prd`, `prod` or `production` as a separate word.
- **Session warnings**: if the current context's credential plugin needs a
  cloud login that isn't active, a warning entry with the fix command
  appears below the contexts. Rechecked every minute.

  | Provider | Check |
  | :--- | :--- |
  | GCP | `gcloud auth print-access-token` |
  | AWS | `aws sts get-caller-identity` (with the plugin's profile) |
  | Azure | `az account get-access-token` (only kubelogin's `azurecli` mode) |

- **Launch at login**: toggles `~/.config/autostart/kubetray.desktop`.

---

## Prerequisites

- **[Go 1.22+](https://go.dev/dl/)** (to build)
- **[`kubectl`](https://kubernetes.io/docs/tasks/tools/)**
- **A StatusNotifierHost** to display the icon, e.g. waybar's `tray`
  module, or `snixembed` if your bar doesn't support StatusNotifierItem.
- The CLI of each cloud you use (`gcloud`, `aws`, `az`) for the session
  warnings.

---

## Installation

```bash
make build                # builds ./kubetray
sudo make install         # installs to /usr/local/bin (PREFIX=/usr to change)
sudo make uninstall
```

Then run `kubetray` and enable "Launch at login" from the menu if you
want it to start with your session.

---

## Configuration

`kubectl` is looked up in `PATH`, then in `~/google-cloud-sdk/bin`,
`~/.local/bin`, `/usr/local/bin` and `/usr/bin` (autostart sessions often
don't have your shell's `PATH`). Set `$KUBECTL` to override.

The icon size is chosen by your tray host (e.g. waybar's `icon-size`);
KubeTray ships the icon at several sizes for it to pick from.

---

## Auxiliary script: `sync-gke.sh`

Helper script to populate your `kubeconfig` with **GKE** clusters. It scans
every Google Cloud project in your organization or account, discovers the
clusters and registers their credentials, skipping contexts that already
exist. KubeTray picks the new contexts up automatically.

> Equivalent scripts for **AWS (EKS)** and **Azure (AKS)** are pending.

**Requirements:** [`gcloud`](https://cloud.google.com/sdk/docs/install),
`kubectl`, Bash 4.0+ and
[`gke-gcloud-auth-plugin`](https://cloud.google.com/blog/products/containers-kubernetes/introducing-gke-gcloud-auth-plugin)
(`gcloud components install gke-gcloud-auth-plugin`).

**What it does:**

- Prompts for `gcloud auth login` if there is no active session.
- Skips system projects (`sys-*`).
- Checks whether `gke_<PROJECT>_<LOCATION>_<CLUSTER>` already exists before
  fetching credentials.
- Keeps going when a project has the Container API disabled or lacks
  permissions.
- Prints a summary of projects scanned, clusters found, contexts added and
  duplicates skipped.

**Usage:**

```bash
chmod +x sync-gke.sh
./sync-gke.sh             # standard sync
./sync-gke.sh --dry-run   # preview without touching kubeconfig
./sync-gke.sh --force     # refresh credentials of existing contexts
```

| Flag | Description |
| :--- | :--- |
| `-d`, `--dry-run` | Shows which clusters would be added without modifying `kubeconfig`. |
| `-f`, `--force` | Fetches credentials even if the context already exists. |
| `-i`, `--internal-ip` | Registers clusters using their private endpoint. |
| `-v`, `--verbose` | Shows detailed debug output. |
| `-h`, `--help` | Displays the help menu. |

---

## License

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
