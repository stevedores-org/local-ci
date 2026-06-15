# Implementation Plan: Remote Execution Feature

## Overview
Add `--remote` flag to local-ci to enable distributed execution via SSH+tmux on aivcs@100.90.209.9

## Architecture

```
local-ci --remote aivcs@100.90.209.9 fmt clippy
         │
         └─→ Detect --remote flag
             │
             ├─→ Check SSH connectivity
             │
             ├─→ Ensure/create tmux session
             │
             └─→ For each stage:
                 ├─→ SSH into remote
                 ├─→ cd to project directory
                 ├─→ Run command in tmux session
                 ├─→ Wrap with exit code sentinel: cmd; echo $? > /tmp/kc_exit_<id>
                 ├─→ Poll sentinel file for exit code
                 └─→ Stream output back to local terminal
```

## Implementation Steps

### Phase 1: Add Flags to main.go
- [x] Add `--remote` flag (SSH host)
- [x] Add `--session` flag (tmux session name, default: "onion")
- [x] Add `--remote-timeout` flag (SSH operation timeout, default: 30s)
- [x] Add `--remote-dir` flag (remote working directory)

### Phase 2: Create remote.go
- [x] `RemoteExecutor` struct with SSH client config
- [x] `ExecuteStage(stage) → Result`
- [x] `runInSession(host, session, cmd) → (output, error)`
- [x] `pollExitCode(host, sentinel) → exitCode`
- [x] Sentinel file strategy: `/tmp/kc_exit_<stage>_<nanos>`

### Phase 3: Modify main.go Execution
- [x] Route stage execution based on --remote flag
- [x] Local execution: current behavior (unchanged)
- [x] Remote execution: new RemoteExecutor path
- [x] Preserve output streaming and result collection
- [x] Workspace sync via rsync before remote stages (#63)
- [x] Named host presets via `--remote-host` (#64)

### Phase 4: Testing
- [x] Unit tests: mock SSH (success, failure, connection error)
- [x] Unit tests: shell quoting + discovery/uranus presets
- [ ] Integration test: Run against live cluster nodes (operator smoke test)

## Files to Create/Modify

| File | Action | LOC |
|------|--------|-----|
| main.go | Add --remote, --session flags + routing logic | ~50 |
| remote.go | NEW: RemoteExecutor implementation | ~200 |
| main_test.go | NEW: tests for remote execution | ~100 |
| CHANGELOG.md | Document new feature | ~10 |

## Key Design Decisions

1. **SSH Session Model:**
   - Use persistent tmux session per --session flag
   - Commands run in same session for workspace context
   - No need to cd for each stage if in correct dir first

2. **Exit Code Strategy (Sentinel File):**
   ```bash
   # Command wrapper on remote
   sh -c '(cmd); echo $? > /tmp/kc_exit_<task_id_hex>'

   # Poll from local
   ssh host cat /tmp/kc_exit_<task_id_hex>  # Blocks until file exists
   ```
   - No subprocess hanging issues
   - Works across SSH
   - Cleans up on completion

3. **Output Streaming:**
   - Use `tmux capture-pane -p` to get output
   - Stream in real-time for --verbose
   - Collect full output for Result

4. **Error Handling:**
   - SSH connection errors: fail fast
   - Stage timeout: kill tmux window/session
   - Exit code sentinel missing: treat as timeout

## Testing Strategy

### Unit Tests
```go
func TestRemoteExecutorSuccess(t *testing.T)     // Mock SSH, verify calls
func TestRemoteExecutorFailure(t *testing.T)     // Non-zero exit code
func TestRemoteExecutorTimeout(t *testing.T)     // Sentinel poll timeout
func TestRemoteExecutorSSHFailure(t *testing.T)  // Connection failure
```

### Integration Test (against aivcs@100.90.209.9)
```bash
cd /tmp/knittingCrab
local-ci --remote aivcs@100.90.209.9 --session test-session fmt clippy
```

## Expected Behavior

### Success Path
```bash
$ local-ci --remote aivcs@100.90.209.9 fmt clippy

🚀 Running local CI pipeline remotely on aivcs@100.90.209.9...

::group::fmt
$ cargo fmt --all -- --check
::endgroup::
✓ fmt (245ms)

::group::clippy
$ cargo clippy --workspace -- -D warnings
::endgroup::
✓ clippy (2345ms)

📊 Summary:
  Total stages: 2
  Passed: 2
  Total time: 2590ms
```

### Failure Path
```bash
$ local-ci --remote aivcs@100.90.209.9 fmt

🚀 Running local CI pipeline remotely on aivcs@100.90.209.9...

::group::fmt
$ cargo fmt --all -- --check
::endgroup::
✗ fmt (145ms)
  Error: formatting issues found

📊 Summary:
  Total stages: 1
  Failed: 1
  Total time: 145ms
```

## Edge Cases Handled

1. **Session already exists:** Reuse existing session
2. **Project directory not on remote:** Error with guidance
3. **Cache directory not writable:** Warning, proceed without cache
4. **SSH key auth fails:** Suggest ssh-add or key setup
5. **Network latency:** Show in output if --verbose
6. **Long-running builds:** Show progress updates every N seconds

## Backward Compatibility

- ✅ No changes to local execution path
- ✅ All existing flags work as-is
- ✅ Default behavior unchanged (local execution)
- ✅ Can mix local and remote in scripts (just different flags)

## Success Criteria

- [ ] `local-ci --remote aivcs@100.90.209.9 fmt` executes on remote
- [ ] Output identical to local execution
- [ ] Exit codes properly captured
- [ ] Cache works on remote
- [ ] Multiple stages execute in sequence
- [ ] Timeout handling works
- [ ] SSH failure handled gracefully
- [ ] All tests pass
