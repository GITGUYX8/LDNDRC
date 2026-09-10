package join

import (
	"context"
	"fmt"
	"time"
)

// RunConfig drives one end-to-end join. Installers are injected so tests
// run the full flow against a fake server without touching the machine.
type RunConfig struct {
	Client      *Client
	Hostname    string
	OS          string
	Arch        string
	HW          Hardware
	JoinKey     string
	Master      Master
	ControlURL  string // http://master:8082 for approval messages
	NeedToolkit bool
	ToolkitFn   func() error
	AgentFn     func(k3sURL, token, nodeName string) error
	Timeout     time.Duration
	Emit        func(string)
	BundleDir   string
}

func (c RunConfig) emit(s string) {
	if c.Emit != nil {
		c.Emit(s)
	}
}

// RunJoin executes register → approval wait → install → join confirm,
// then writes the diagnostics bundle (always local; uploaded only when the
// polls proved the network healthy). It returns the Done-screen message.
func RunJoin(ctx context.Context, cfg RunConfig) (string, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Minute
	}
	emit := cfg.emit

	emit("registering with master " + cfg.Master.Addr())
	id, err := cfg.Client.Register(ctx, Fingerprint{
		Hostname: cfg.Hostname, OS: cfg.OS, Arch: cfg.Arch,
		CPU: cfg.HW.CPU, RAMGB: cfg.HW.RAMGB, GPU: cfg.HW.GPU, JoinKey: cfg.JoinKey,
	})
	if err != nil {
		return failWithBundle(cfg, err, []string{"register failed: " + err.Error()})
	}
	emit("registered as " + id)
	emit(fmt.Sprintf("waiting for operator approval at %s (polling every 5s)", cfg.ControlURL))

	res, err := cfg.Client.WaitApproval(ctx, id, timeout)
	networkHealthy := err == nil
	if err != nil {
		return failWithBundle(cfg, err, []string{"registered as " + id, "approval wait: " + err.Error()})
	}
	steps := []string{"registered as " + id, "approved"}
	bundle := Bundle{Hostname: cfg.Hostname, OS: cfg.OS, Steps: steps}

	switch res.Status {
	case "denied":
		msg := "operator denied the request"
		if res.Reason != "" {
			msg += ": " + res.Reason
		}
		return failWithBundle(cfg, fmt.Errorf("%s", msg), append(steps, msg))
	case "joined":
		return doneWithBundle(cfg, id, bundle, append(steps, "already joined"), networkHealthy)
	}
	if res.K3sToken == "" || res.K3sURL == "" || res.NodeName == "" {
		err := fmt.Errorf("approved but no token payload (is K3S_JOIN_URL set on the master?)")
		return failWithBundle(cfg, err, append(steps, err.Error()))
	}
	steps = append(steps, "short-lived token received (memory only, never shown)")

	toolkit := cfg.ToolkitFn
	if toolkit == nil {
		toolkit = InstallToolkit
	}
	if cfg.NeedToolkit {
		emit("installing NVIDIA container toolkit (consented)")
		if err := toolkit(); err != nil {
			return failWithBundle(cfg, err, append(steps, "toolkit install: "+err.Error()))
		}
		steps = append(steps, "toolkit installed")
	}
	agent := cfg.AgentFn
	if agent == nil {
		agent = InstallAgent
	}
	emit("installing k3s agent as " + res.NodeName)
	if err := agent(res.K3sURL, res.K3sToken, res.NodeName); err != nil {
		return failWithBundle(cfg, err, append(steps, "agent install: "+err.Error()))
	}
	steps = append(steps, "agent installed, waiting for Ready + label")
	emit("agent installed, waiting for the master to confirm Ready + label")
	if err := cfg.Client.WaitJoined(ctx, id, timeout); err != nil {
		return failWithBundle(cfg, err, append(steps, err.Error()))
	}
	steps = append(steps, "joined")
	return doneWithBundle(cfg, id, bundle, steps, networkHealthy)
}

func failWithBundle(cfg RunConfig, err error, steps []string) (string, error) {
	bundle := Bundle{Hostname: cfg.Hostname, OS: cfg.OS, Steps: steps}
	path, werr := bundle.WriteLocal(cfg.BundleDir)
	suffix := ""
	if werr == nil {
		suffix = " diagnostics: " + path
	}
	return "failed: " + err.Error() + suffix, err
}

func doneWithBundle(cfg RunConfig, id string, bundle Bundle, steps []string, networkHealthy bool) (string, error) {
	bundle.Steps = steps
	path, werr := bundle.WriteLocal(cfg.BundleDir)
	uploaded := ""
	if werr == nil {
		if uerr := cfg.Client.MaybeUpload(context.Background(), id, bundle.Render(), networkHealthy); uerr == nil {
			uploaded = " diagnostics uploaded."
		} else {
			uploaded = " diagnostics kept locally (" + path + ") — upload failed, share it manually."
		}
	}
	msg := fmt.Sprintf("joined! This laptop is now a cluster Host (node ldndrc-%s)."+uploaded+" Leave anytime with k3s-uninstall.sh.",
		id)
	return msg, nil
}
