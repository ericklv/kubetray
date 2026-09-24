// Package kubecontext reads and switches the current kubeconfig
// context via the kubectl CLI, rather than editing kubeconfig YAML
// directly, so we inherit kubectl's handling of multi-file KUBECONFIG
// merges (and stay correct if that logic changes).
package kubecontext

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Context is one kubeconfig context. For contexts created by
// `gcloud container clusters get-credentials` (named
// gke_<project>_<location>_<cluster>), the GKE fields are filled in;
// for anything else only Name is set.
type Context struct {
	Name     string
	Project  string
	Location string
	Cluster  string
}

// IsGKE reports whether the context name followed the gcloud naming
// convention and was split into project/location/cluster.
func (c Context) IsGKE() bool { return c.Project != "" }

// Parse splits a gke_<project>_<location>_<cluster> name. GCP
// project ids, locations and cluster names can't contain underscores,
// so splitting into exactly four parts is unambiguous.
func Parse(name string) Context {
	c := Context{Name: name}
	parts := strings.SplitN(name, "_", 4)
	if len(parts) == 4 && parts[0] == "gke" && parts[1] != "" && parts[2] != "" && parts[3] != "" {
		c.Project, c.Location, c.Cluster = parts[1], parts[2], parts[3]
	}
	return c
}

// kubectlPath resolves the kubectl binary. When launched from an XDG
// autostart entry the session PATH often lacks shell-profile additions
// like ~/google-cloud-sdk/bin, so fall back to common install spots.
// $KUBECTL overrides everything.
func kubectlPath() (string, error) {
	if p := os.Getenv("KUBECTL"); p != "" {
		return p, nil
	}
	if p, err := exec.LookPath("kubectl"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(home, "google-cloud-sdk", "bin", "kubectl"),
		filepath.Join(home, ".local", "bin", "kubectl"),
		"/usr/local/bin/kubectl",
		"/usr/bin/kubectl",
	} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("kubectl not found in PATH or common install locations (set $KUBECTL)")
}

// kubectl runs a `kubectl config ...` subcommand and returns stdout.
// stderr is only surfaced on failure: with an expired gcloud session,
// kubectl prints credential-plugin noise there even for purely local
// config commands that otherwise succeed.
func kubectl(args ...string) (string, error) {
	bin, err := kubectlPath()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(bin, append([]string{"config"}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl config %s: %w (%s)", strings.Join(args, " "), err, lastLine(stderr.String()))
	}
	return string(out), nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// List returns every context in the merged kubeconfig.
func List() ([]Context, error) {
	out, err := kubectl("get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}
	var result []Context
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			result = append(result, Parse(name))
		}
	}
	return result, nil
}

// Current returns the name of the current context, or "" if none is set.
func Current() (string, error) {
	out, err := kubectl("current-context")
	if err != nil {
		// kubectl exits non-zero when current-context is simply unset;
		// that's not worth reporting as an error.
		if strings.Contains(err.Error(), "current-context is not set") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Use switches the current context.
func Use(name string) error {
	_, err := kubectl("use-context", name)
	return err
}

// ConfigFiles returns the kubeconfig files kubectl reads, in the same
// order kubectl does: each entry of $KUBECONFIG, else ~/.kube/config.
// Used to watch for context changes made outside the tray.
func ConfigFiles() []string {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		var files []string
		for _, f := range filepath.SplitList(env) {
			if f != "" {
				files = append(files, f)
			}
		}
		return files
	}
	home, _ := os.UserHomeDir()
	return []string{filepath.Join(home, ".kube", "config")}
}
