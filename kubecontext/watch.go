package kubecontext

import (
	"os"
	"time"
)

// fileStamp is enough to notice a rewrite: kubectl and gcloud replace
// the whole file on every change, which bumps mtime (and usually size).
type fileStamp struct {
	modTime time.Time
	size    int64
	exists  bool
}

func stamps(files []string) []fileStamp {
	out := make([]fileStamp, len(files))
	for i, f := range files {
		if info, err := os.Stat(f); err == nil {
			out[i] = fileStamp{modTime: info.ModTime(), size: info.Size(), exists: true}
		}
	}
	return out
}

// Watch polls the kubeconfig files every interval and calls onChange
// whenever one of them changes, so a `kubectl config use-context` run
// from a terminal (or a new cluster added by sync-gke.sh) shows up in
// the tray. Polling instead of inotify keeps us dependency-free, and
// catches the atomic rename kubectl does on write without having to
// re-arm watches. Never returns.
func Watch(interval time.Duration, onChange func()) {
	files := ConfigFiles()
	last := stamps(files)
	for {
		time.Sleep(interval)
		cur := stamps(files)
		changed := false
		for i := range cur {
			if cur[i] != last[i] {
				changed = true
				break
			}
		}
		last = cur
		if changed {
			onChange()
		}
	}
}
