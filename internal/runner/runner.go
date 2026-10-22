package runner

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"qahyacli/internal/jobapi"
	"qahyacli/internal/state"
)

// Run executes the jobs steps in order and stops at the first failure.
func Run(ctx context.Context, job *Job, store *state.Store, socket string) error {
	// steps call qahyacli by name, so the dire of this very binary goes first on PATH
	self, err := os.Executable()
	if err != nil { return fmt.Errorf("find own binary: %w", err) }

	binDir := filepath.Dir(self)

	for i, step := range job.Steps {
		fmt.Fprintf(os.Stderr, "==> [%d/%d] %s\n", i+1, len(job.Steps), step.Name)

		env, err := stepEnv(store, socket, binDir, step.Name)
		if err != nil { return fmt.Errorf("step %q: %w", step.Name, err) }
		
		cmd := exec.CommandContext(ctx, "sh", "-c", step.Run)
		cmd.Dir = job.Dir
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
	}
	fmt.Fprintln(os.Stderr, "==> job succeeded")
	return nil
}

// stepEnv builds a steps environment. nothing is inherited implicitly: if
// cmd.Env were nil the step would get the runner's entire environment.
func stepEnv(store *state.Store, socket, binDir, stepName string) ([]string, error) {
	env := []string{
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		jobapi.SocketEnv + "=" + socket,
		jobapi.StepEnv + "=" + stepName,
	}

	snap := store.Snapshot()
	for _, name := range slices.Sorted(maps.Keys(snap)) {
		// secrets are fetched here: the "fetch late" half of the middle approach
		v, err := snap[name].Resolve()
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", name, err)
		}
		env = append(env, name+"="+v)
	}
	return env, nil
}
