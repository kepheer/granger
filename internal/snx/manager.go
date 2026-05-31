package snx

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"

	"granger/internal/config"
	"granger/pkg/runner"
)

const (
	defaultPromptTimeout = 30 * time.Second
	defaultSubmitTimeout = 90 * time.Second
)

type Manager struct {
	mu      sync.Mutex
	pending *PendingConnect
	Runner  runner.Runner
}

type PendingConnect struct {
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	ptmx    *os.File
	buf     *runner.SafeBuffer
	done    chan error
	fin     chan struct{}
	flow    []config.AuthStep
	step    int
	started time.Time
}

type PendingStatus struct {
	Pending     bool   `json:"pending"`
	Step        string `json:"step,omitempty"`
	StepType    string `json:"step_type,omitempty"`
	Label       string `json:"label,omitempty"`
	Output      string `json:"output,omitempty"`
	StartedUnix int64  `json:"started_unix,omitempty"`
}

func New(r runner.Runner) *Manager { return &Manager{Runner: r} }

func DefaultAuthFlow() []config.AuthStep {
	return []config.AuthStep{
		{Type: "password", Label: "Password", Secret: true, TimeoutMS: int(defaultPromptTimeout / time.Millisecond)},
		{Type: "sms", Label: "Verification code", Secret: true, TimeoutMS: int(defaultSubmitTimeout / time.Millisecond)},
	}
}

func ReadUsername(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`(?m)^\s*user-name\s*=\s*"?([^"\n#]+)"?`)
	m := re.FindStringSubmatch(string(b))
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func UpdateUsername(path, username string) runner.Result {
	title := "Update SNX username"
	username = strings.TrimSpace(username)
	if username == "" {
		return runner.Result{Title: title, Command: path, Output: "username is empty", OK: false, Status: "error"}
	}
	if strings.ContainsAny(username, "\r\n") {
		return runner.Result{Title: title, Command: path, Output: "username must be a single line", OK: false, Status: "error"}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return runner.Result{Title: title, Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	lines := strings.Split(string(b), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "user-name") {
			lines[i] = `user-name = "` + escape(username) + `"`
			found = true
		}
	}
	if !found {
		lines = append(lines, `user-name = "`+escape(username)+`"`)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return runner.Result{Title: title, Command: path, Output: err.Error(), OK: false, Status: "error"}
	}
	return runner.Result{Title: title, Command: path, Output: "user-name updated", OK: true, Status: "ok"}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

func (m *Manager) Start(up config.Upstream, inputs map[string]string) runner.Result {
	m.Cancel()
	flow := normalizeFlow(up.AuthFlow)
	if username := strings.TrimSpace(inputs["username"]); username != "" && up.Config != "" {
		res := UpdateUsername(up.Config, username)
		if !res.OK {
			return res
		}
	}
	passwordStep := firstSecretInputStep(flow)
	if passwordStep < 0 {
		return runner.Result{Title: "Start SNX-RS auth", Command: "auth_flow", Output: "auth_flow must contain a secret input step", OK: false, Status: "error"}
	}
	value := strings.TrimSpace(inputValue(inputs, flow[passwordStep]))
	if value == "" {
		return runner.Result{Title: "Start SNX-RS auth", Command: "snxctl connect", Output: "first secret value is empty", OK: false, Status: "error"}
	}
	_ = m.Runner.RunSoft("Disconnect previous SNX-RS session", 20*time.Second, nil, "snxctl", "disconnect")

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "snxctl", "connect")
	buf := &runner.SafeBuffer{}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return runner.Result{Title: "Start SNX-RS auth", Command: "snxctl connect", Output: err.Error(), OK: false, Status: "error"}
	}
	p := &PendingConnect{
		cmd: cmd, cancel: cancel, ptmx: ptmx, buf: buf,
		done: make(chan error, 1), fin: make(chan struct{}),
		flow: flow, step: passwordStep, started: time.Now(),
	}
	go copyPTYOutput(ptmx, buf)
	go p.wait()

	if !waitForPrompt(p, promptPatterns(up, flow[passwordStep]), stepTimeout(flow[passwordStep], defaultPromptTimeout)) {
		return m.finishPromptFailure(p, "Start SNX-RS auth", "timeout waiting for first prompt")
	}
	if _, err := ptmx.Write([]byte(value + "\n")); err != nil {
		p.kill()
		return runner.Result{Title: "Start SNX-RS auth", Command: "snxctl connect", Output: err.Error(), OK: false, Status: "error"}
	}

	next := nextInputStep(flow, passwordStep+1)
	if next < 0 {
		return m.waitFinished(p, "Finish SNX-RS auth", defaultSubmitTimeout)
	}
	if !waitForPrompt(p, promptPatterns(up, flow[next]), stepTimeout(flow[next], defaultPromptTimeout)) {
		return m.finishPromptFailure(p, "Start SNX-RS auth", "timeout waiting for next prompt")
	}
	p.step = next
	m.mu.Lock()
	m.pending = p
	m.mu.Unlock()
	return runner.Result{Title: "Start SNX-RS auth", Command: "snxctl connect", Output: SanitizeOutput(buf.String()), OK: true, Status: "pending:" + flow[next].Type}
}

func (m *Manager) Submit(inputs map[string]string) runner.Result {
	m.mu.Lock()
	p := m.pending
	m.mu.Unlock()
	if p == nil {
		return runner.Result{Title: "Submit SNX-RS auth", Command: "snxctl connect", Output: "no pending SNX-RS process", OK: false, Status: "error"}
	}
	step := p.flow[p.step]
	value := strings.TrimSpace(inputValue(inputs, step))
	if value == "" {
		return runner.Result{Title: "Submit SNX-RS auth", Command: "snxctl connect", Output: "auth value is empty", OK: false, Status: "error"}
	}
	if _, err := p.ptmx.Write([]byte(value + "\n")); err != nil {
		m.clearPending(p)
		p.kill()
		return runner.Result{Title: "Submit SNX-RS auth", Command: "snxctl connect", Output: err.Error(), OK: false, Status: "error"}
	}
	next := nextInputStep(p.flow, p.step+1)
	if next < 0 {
		m.clearPending(p)
		return m.waitFinished(p, "Finish SNX-RS auth", defaultSubmitTimeout)
	}
	if !waitForPrompt(p, p.flow[next].Prompts, stepTimeout(p.flow[next], defaultPromptTimeout)) {
		m.clearPending(p)
		return m.finishPromptFailure(p, "Submit SNX-RS auth", "timeout waiting for next prompt")
	}
	p.step = next
	return runner.Result{Title: "Submit SNX-RS auth", Command: "snxctl connect", Output: SanitizeOutput(p.buf.String()), OK: true, Status: "pending:" + p.flow[next].Type}
}

func (m *Manager) Disconnect() runner.Result {
	m.Cancel()
	return m.Runner.Run("Disconnect SNX-RS", 25*time.Second, nil, "snxctl", "disconnect")
}

func (m *Manager) Cancel() {
	m.mu.Lock()
	p := m.pending
	m.pending = nil
	m.mu.Unlock()
	if p != nil {
		p.kill()
	}
}

func (m *Manager) PendingStatus() PendingStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending == nil {
		return PendingStatus{}
	}
	step := m.pending.flow[m.pending.step]
	return PendingStatus{
		Pending: true, Step: stepName(step), StepType: step.Type, Label: step.Label,
		Output: SanitizeOutput(m.pending.buf.String()), StartedUnix: m.pending.started.Unix(),
	}
}

func (m *Manager) clearPending(p *PendingConnect) {
	m.mu.Lock()
	if m.pending == p {
		m.pending = nil
	}
	m.mu.Unlock()
}

func (m *Manager) finishPromptFailure(p *PendingConnect, title, status string) runner.Result {
	select {
	case err := <-p.done:
		p.cleanup()
		return runner.Result{Title: title, Command: "snxctl connect", Output: SanitizeOutput(p.buf.String()), OK: err == nil, Status: finishStatus(err)}
	default:
		p.kill()
		return runner.Result{Title: title, Command: "snxctl connect", Output: SanitizeOutput(p.buf.String()), OK: false, Status: status}
	}
}

func (m *Manager) waitFinished(p *PendingConnect, title string, timeout time.Duration) runner.Result {
	select {
	case err := <-p.done:
		p.cleanup()
		return runner.Result{Title: title, Command: "snxctl connect", Output: SanitizeOutput(p.buf.String()), OK: err == nil, Status: finishStatus(err)}
	case <-time.After(timeout):
		p.kill()
		return runner.Result{Title: title, Command: "snxctl connect", Output: SanitizeOutput(p.buf.String()), OK: false, Status: "timeout"}
	}
}

func (p *PendingConnect) wait() {
	err := p.cmd.Wait()
	p.done <- err
	close(p.fin)
}

func (p *PendingConnect) kill() {
	p.cancel()
	_ = p.ptmx.Close()
}

func (p *PendingConnect) cleanup() {
	p.cancel()
	_ = p.ptmx.Close()
}

func copyPTYOutput(r io.Reader, b *runner.SafeBuffer) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			_, _ = b.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func waitForPrompt(p *PendingConnect, prompts []string, timeout time.Duration) bool {
	if len(prompts) == 0 {
		prompts = defaultPromptsForType(p.flow[p.step].Type)
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			return false
		case <-p.fin:
			return false
		case <-tick.C:
			out := strings.ToLower(SanitizeOutput(p.buf.String()))
			for _, prompt := range prompts {
				if strings.Contains(out, strings.ToLower(prompt)) {
					return true
				}
			}
		}
	}
}

func normalizeFlow(flow []config.AuthStep) []config.AuthStep {
	if len(flow) == 0 {
		flow = DefaultAuthFlow()
	}
	out := make([]config.AuthStep, len(flow))
	copy(out, flow)
	for i := range out {
		if out[i].Name == "" {
			out[i].Name = out[i].Type
		}
		if out[i].Label == "" {
			out[i].Label = out[i].Name
		}
	}
	return out
}

func promptPatterns(up config.Upstream, step config.AuthStep) []string {
	if len(step.Prompts) > 0 {
		return step.Prompts
	}
	if up.PromptPatterns != nil {
		if prompts := up.PromptPatterns[step.Type]; len(prompts) > 0 {
			return prompts
		}
	}
	return defaultPromptsForType(step.Type)
}

func defaultPromptsForType(typ string) []string {
	switch typ {
	case "password":
		return []string{"Password:", "Пароль:", "Укажите имя пользователя и пароль"}
	case "sms":
		return []string{"Verification Code:", "verification code", "SMS", "sms", "код"}
	case "email":
		return []string{"Verification Code:", "verification code", "Email", "E-mail", "code"}
	case "otp":
		return []string{"OTP", "TOTP", "verification code", "Verification Code:", "code"}
	default:
		return []string{typ, "code", "input"}
	}
}

func firstSecretInputStep(flow []config.AuthStep) int {
	for i, step := range flow {
		if step.Type == "password" || step.Secret {
			return i
		}
	}
	return -1
}

func nextInputStep(flow []config.AuthStep, start int) int {
	for i := start; i < len(flow); i++ {
		switch flow[i].Type {
		case "password", "otp", "sms", "email", "custom":
			return i
		}
	}
	return -1
}

func inputValue(inputs map[string]string, step config.AuthStep) string {
	if inputs == nil {
		return ""
	}
	for _, key := range []string{step.Name, step.Type, "value", "code"} {
		if key != "" {
			if value := inputs[key]; value != "" {
				return value
			}
		}
	}
	return ""
}

func stepName(step config.AuthStep) string {
	if step.Name != "" {
		return step.Name
	}
	return step.Type
}

func stepTimeout(step config.AuthStep, fallback time.Duration) time.Duration {
	if step.TimeoutMS > 0 {
		return time.Duration(step.TimeoutMS) * time.Millisecond
	}
	return fallback
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func SanitizeOutput(s string) string {
	s = ansiRE.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func finishStatus(err error) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "error: " + err.Error()
}
