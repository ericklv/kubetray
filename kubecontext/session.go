package kubecontext

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

const sessionCheckTimeout = 10 * time.Second

type execConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
}

// argValue returns the value following any of flags in args.
func argValue(args []string, flags ...string) string {
	for i, a := range args {
		if slices.Contains(flags, a) && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// SessionWarning checks whether the cloud CLI backing the context's
// credential plugin has an active session. It returns a
// user-facing warning, or "" when the session is fine or the context
// doesn't rely on a cloud login (client certs, static tokens...).
func SessionWarning(c Context) string {
	ex := c.exec
	if ex == nil {
		return ""
	}

	var provider, fix string
	var check []string
	switch filepath.Base(ex.Command) {
	case "gke-gcloud-auth-plugin", "gcloud":
		// print-access-token fails when the refresh token needs reauth,
		// which `gcloud auth list` would still report as ACTIVE.
		provider, fix = "GCP", "gcloud auth login"
		check = []string{"gcloud", "auth", "print-access-token"}
	case "aws", "aws-iam-authenticator":
		provider, fix = "AWS", "aws sso login"
		check = []string{"aws", "sts", "get-caller-identity"}
		if profile := argValue(ex.Args, "--profile"); profile != "" {
			check = append(check, "--profile", profile)
			fix += " --profile " + profile
		}
	case "kubelogin":
		// Only the azurecli login mode reuses the az session; the other
		// modes keep their own token cache we can't probe without prompting.
		if argValue(ex.Args, "-l", "--login") != "azurecli" {
			return ""
		}
		provider, fix = "Azure", "az login"
		check = []string{"az", "account", "get-access-token"}
	default:
		return ""
	}

	bin, err := findBinary(check[0])
	if err != nil {
		return "⚠ " + provider + ": " + check[0] + " not found"
	}
	ctx, cancel := context.WithTimeout(context.Background(), sessionCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, check[1:]...)
	// The plugin's env carries settings like AWS_PROFILE that pick
	// which session kubectl will actually use.
	cmd.Env = os.Environ()
	for _, e := range ex.Env {
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	if cmd.Run() != nil {
		return "⚠ No active " + provider + " session — run: " + fix
	}
	return ""
}
