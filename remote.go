package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// remoteSSH abstracts SSH execution so RemoteExecutor can be unit-tested
// without a live host. When nil on RemoteExecutor, execSSH is used.
type remoteSSH interface {
	execWithOutput(ctx context.Context, cmd string) (string, error)
}

type execSSH struct {
	host           string
	connectTimeout time.Duration
}

func (e execSSH) execWithOutput(ctx context.Context, cmd string) (string, error) {
	timeoutSec := int(e.connectTimeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 10
	}
	sshCmd := exec.CommandContext(
		ctx,
		"ssh",
		"-o",
		fmt.Sprintf("ConnectTimeout=%d", timeoutSec),
		e.host,
		cmd,
	)

	var stdout, stderr bytes.Buffer
	sshCmd.Stdout = &stdout
	sshCmd.Stderr = &stderr

	if err := sshCmd.Run(); err != nil {
		if strings.Contains(cmd, "cat") && strings.Contains(stderr.String(), "No such file") {
			return "", nil
		}
		if stderr.Len() == 0 && stdout.Len() == 0 {
			return "", nil
		}
		return stdout.String(), fmt.Errorf("SSH command failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

// RemoteExecutor handles execution of stages on a remote machine via SSH+tmux
type RemoteExecutor struct {
	Host    string        // SSH host (e.g., "aivcs@100.90.209.9")
	Session string        // tmux session name (e.g., "onion")
	WorkDir string        // Remote working directory
	Timeout time.Duration // SSH operation timeout
	Verbose bool          // Show detailed output
	ssh     remoteSSH     // test hook; defaults to execSSH
}

// NewRemoteExecutor creates a new remote executor
func NewRemoteExecutor(host, session, workDir string, timeout time.Duration, verbose bool) *RemoteExecutor {
	if session == "" {
		session = "onion"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &RemoteExecutor{
		Host:    host,
		Session: session,
		WorkDir: workDir,
		Timeout: timeout,
		Verbose: verbose,
	}
}

func (re *RemoteExecutor) sshClient() remoteSSH {
	if re.ssh != nil {
		return re.ssh
	}
	return execSSH{host: re.Host, connectTimeout: re.Timeout}
}

// joinShellCommand quotes argv for safe remote shell execution.
func joinShellCommand(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = escapeShellArg(p)
	}
	return strings.Join(quoted, " ")
}

// buildRemoteStageCommand wraps a stage command with cd + exit-code sentinel capture.
func buildRemoteStageCommand(workDir string, cmd []string, sentinelFile string) string {
	return fmt.Sprintf(
		"cd %s && %s; echo $? > %s",
		escapeShellArg(workDir),
		joinShellCommand(cmd),
		sentinelFile,
	)
}

// ExecuteStage runs a single stage on the remote machine
func (re *RemoteExecutor) ExecuteStage(stage Stage) Result {
	start := time.Now()
	result := Result{
		Name:    stage.Name,
		Command: strings.Join(stage.Cmd, " "),
		Status:  "fail",
	}

	sentinelFile := fmt.Sprintf("/tmp/kc_exit_%s_%d", stage.Name, time.Now().UnixNano())
	remoteCmd := buildRemoteStageCommand(re.WorkDir, stage.Cmd, sentinelFile)

	stageTimeout := time.Duration(stage.Timeout) * time.Second
	if stageTimeout <= 0 {
		stageTimeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), stageTimeout)
	defer cancel()

	output, err := re.runInSession(ctx, remoteCmd)
	if err != nil {
		result.Error = err
		result.Output = output
		result.Duration = time.Since(start)
		if re.Verbose {
			warnf("Remote execution failed: %v", err)
		}
		return result
	}

	exitCode, err := re.pollExitCode(ctx, sentinelFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to get exit code: %w", err)
		result.Output = output
		result.Duration = time.Since(start)
		return result
	}

	_ = re.cleanupSentinel(sentinelFile)

	result.Output = output
	result.Duration = time.Since(start)

	if exitCode == 0 {
		result.Status = "pass"
	} else {
		result.Status = "fail"
		result.Error = fmt.Errorf("exit code %d", exitCode)
	}

	return result
}

// runInSession executes a command within a tmux session
func (re *RemoteExecutor) runInSession(ctx context.Context, cmd string) (string, error) {
	initCmd := fmt.Sprintf(
		"tmux new-session -d -s %s -c %s 'sleep 999999' 2>/dev/null; true",
		re.Session,
		escapeShellArg(re.WorkDir),
	)

	if err := re.sshExec(ctx, initCmd); err != nil {
		if re.Verbose {
			warnf("Warning: could not initialize session: %v (proceeding anyway)", err)
		}
	}

	sendCmd := fmt.Sprintf(
		"tmux send-keys -t %s '%s' Enter",
		re.Session,
		escapeForTmux(cmd),
	)

	if err := re.sshExec(ctx, sendCmd); err != nil {
		return "", fmt.Errorf("failed to send command to tmux: %w", err)
	}

	time.Sleep(100 * time.Millisecond)

	captureCmd := fmt.Sprintf("tmux capture-pane -t %s -p", re.Session)
	output, err := re.sshExecWithOutput(ctx, captureCmd)
	if err != nil {
		return "", fmt.Errorf("failed to capture output: %w", err)
	}

	return output, nil
}

// pollExitCode polls the remote sentinel file for the exit code
func (re *RemoteExecutor) pollExitCode(ctx context.Context, sentinelFile string) (int, error) {
	maxRetries := 300
	retries := 0

	for retries < maxRetries {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		default:
		}

		catCmd := fmt.Sprintf("cat %s 2>/dev/null", sentinelFile)
		output, err := re.sshExecWithOutput(ctx, catCmd)
		if err == nil && output != "" {
			code, err := strconv.Atoi(strings.TrimSpace(output))
			if err == nil {
				return code, nil
			}
		}

		time.Sleep(100 * time.Millisecond)
		retries++
	}

	return -1, fmt.Errorf("timeout waiting for exit code from %s", sentinelFile)
}

// cleanupSentinel removes the sentinel file
func (re *RemoteExecutor) cleanupSentinel(sentinelFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), re.Timeout)
	defer cancel()

	cleanupCmd := fmt.Sprintf("rm -f %s", sentinelFile)
	return re.sshExec(ctx, cleanupCmd)
}

func (re *RemoteExecutor) sshExec(ctx context.Context, cmd string) error {
	_, err := re.sshExecWithOutput(ctx, cmd)
	return err
}

func (re *RemoteExecutor) sshExecWithOutput(ctx context.Context, cmd string) (string, error) {
	return re.sshClient().execWithOutput(ctx, cmd)
}

// escapeShellArg safely escapes a shell argument
func escapeShellArg(arg string) string {
	if !strings.ContainsAny(arg, " \t\n'\"\\$`") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

// escapeForTmux safely escapes a command for tmux send-keys
func escapeForTmux(cmd string) string {
	return strings.ReplaceAll(cmd, "'", "'\\''")
}

// EnsureRemoteSession creates or attaches to a remote tmux session
func (re *RemoteExecutor) EnsureRemoteSession(ctx context.Context) error {
	cmd := fmt.Sprintf(
		"tmux new-session -d -s %s -c %s 'sleep 999999' 2>/dev/null || true",
		re.Session,
		escapeShellArg(re.WorkDir),
	)
	return re.sshExec(ctx, cmd)
}

// KillRemoteSession kills the remote tmux session
func (re *RemoteExecutor) KillRemoteSession(ctx context.Context) error {
	cmd := fmt.Sprintf("tmux kill-session -t %s 2>/dev/null || true", re.Session)
	return re.sshExec(ctx, cmd)
}

// TestSSHConnection tests if SSH connection works
func (re *RemoteExecutor) TestSSHConnection(ctx context.Context) error {
	return re.sshExec(ctx, "echo 'SSH connection OK'")
}

// SyncWorkspace uses rsync to synchronize local directory to remote WorkDir
func (re *RemoteExecutor) SyncWorkspace(ctx context.Context, localDir string, skipDirs []string) error {
	mkdirCmd := fmt.Sprintf("mkdir -p %s", escapeShellArg(re.WorkDir))
	if err := re.sshExec(ctx, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create remote work directory: %w", err)
	}

	rsyncArgs := []string{"-az", "--delete"}
	rsyncArgs = append(rsyncArgs, "--exclude", ".git", "--exclude", ".local-ci-cache")

	for _, dir := range skipDirs {
		if dir != "" && dir != ".git" {
			rsyncArgs = append(rsyncArgs, "--exclude", dir)
		}
	}

	src := localDir
	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	dest := fmt.Sprintf("%s:%s", re.Host, re.WorkDir)
	rsyncArgs = append(rsyncArgs, src, dest)

	if re.Verbose {
		printf("Syncing workspace to remote: rsync %s\n", strings.Join(rsyncArgs, " "))
	}

	cmd := exec.CommandContext(ctx, "rsync", rsyncArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}
