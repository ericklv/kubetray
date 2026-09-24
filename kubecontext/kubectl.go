// Package kubecontext reads and switches the current kubeconfig
// context via the kubectl CLI, rather than editing kubeconfig YAML
// directly, so we inherit kubectl's handling of multi-file KUBECONFIG
// merges (and stay correct if that logic changes).
package kubecontext

import (
	"encoding/json"
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

	exec *execConfig // credential plugin of the context's user, if any
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

// findBinary resolves a CLI by name. When launched from an XDG
// autostart entry the session PATH often lacks shell-profile additions
// like ~/google-cloud-sdk/bin, so fall back to common install spots.
func findBinary(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{
		filepath.Join(home, "google-cloud-sdk", "bin"),
		filepath.Join(home, ".local", "bin"),
		"/usr/local/bin",
		"/usr/bin",
	} {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH or common install locations", name)
}

func kubectlPath() (string, error) {
	if p := os.Getenv("KUBECTL"); p != "" {
		return p, nil
	}
	return findBinary("kubectl")
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

// Config is the subset of the merged kubeconfig the tray needs.
type Config struct {
	Current  string
	Contexts []Context
}

// Load reads the merged kubeconfig with a single kubectl call; each
// call costs ~1s when kubectl runs credential plugins on startup.
func Load() (Config, error) {
	out, err := kubectl("view", "-o", "json")
	if err != nil {
		return Config{}, err
	}
	var raw struct {
		Current  string `json:"current-context"`
		Contexts []struct {
			Name    string `json:"name"`
			Context struct {
				User string `json:"user"`
			} `json:"context"`
		} `json:"contexts"`
		Users []struct {
			Name string `json:"name"`
			User struct {
				Exec *execConfig `json:"exec"`
			} `json:"user"`
		} `json:"users"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return Config{}, fmt.Errorf("parsing kubeconfig: %w", err)
	}
	execs := make(map[string]*execConfig, len(raw.Users))
	for _, u := range raw.Users {
		execs[u.Name] = u.User.Exec
	}
	cfg := Config{Current: raw.Current}
	for _, c := range raw.Contexts {
		ctx := Parse(c.Name)
		ctx.exec = execs[c.Context.User]
		cfg.Contexts = append(cfg.Contexts, ctx)
	}
	return cfg, nil
}

// Find returns the context with the given name.
func (c Config) Find(name string) (Context, bool) {
	for _, ctx := range c.Contexts {
		if ctx.Name == name {
			return ctx, true
		}
	}
	return Context{}, false
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
