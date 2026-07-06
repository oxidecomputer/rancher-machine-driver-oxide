// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// applyAttempts is the number of times a stage's `terraform apply` is
	// attempted before giving up.
	applyAttempts = 3

	// applyRetryDelay is how long to wait between apply attempts.
	applyRetryDelay = 1 * time.Minute
)

// terraformStages are the Terraform configurations under
// `oxidecomputer/showcase/integration/rancher`, in dependency order. The
// `image` stage is intentionally excluded because it is Packer, not Terraform.
var terraformStages = []string{
	"rke2",
	"rancher",
	"nodedriver",
	"k8s",
	"longhorn",
}

func TestRancherNodeDriver(t *testing.T) {
	rancherDir, terraformBin := e2eSetup(t)

	// In CI the destroy runs as a dedicated `if: always()` workflow step so
	// that it also executes when this test is cancelled or times out (neither
	// of which run t.Cleanup). Set RANCHER_E2E_SKIP_DESTROY=1 there to avoid a
	// redundant in-test destroy. Local runs keep the cleanup for convenience.
	skipDestroy := os.Getenv("RANCHER_E2E_SKIP_DESTROY") != ""

	for _, stage := range terraformStages {
		stageDir := filepath.Join(rancherDir, stage)

		extraEnv := stageEnv(stage)

		if !skipDestroy {
			t.Cleanup(func() {
				if err := terraform(t, terraformBin, stageDir, extraEnv,
					"destroy", "-input=false", "-auto-approve"); err != nil {
					t.Errorf("stage %q destroy: %v", stage, err)
				}
			})
		}

		ok := t.Run(stage, func(t *testing.T) {
			if err := terraform(t, terraformBin, stageDir, extraEnv, "init", "-input=false"); err != nil {
				t.Fatalf("init: %v", err)
			}

			var err error
			for attempt := 1; attempt <= applyAttempts; attempt++ {
				err = terraform(t, terraformBin, stageDir, extraEnv,
					"apply", "-input=false", "-auto-approve")
				if err == nil {
					break
				}
				if attempt < applyAttempts {
					t.Logf("apply attempt %d/%d failed: %v; retrying in %s",
						attempt, applyAttempts, err, time.Duration(attempt)*applyRetryDelay)
					time.Sleep(time.Duration(attempt) * applyRetryDelay)
				}
			}
			if err != nil {
				t.Fatalf("apply failed after %d attempts: %v", applyAttempts, err)
			}
		})
		if !ok {
			t.Fatalf("stage %q failed; halting pipeline", stage)
		}
	}
}

// TestRancherNodeDriverDestroy tears down every Terraform stage in reverse
// dependency order. It is a standalone entry point so CI can run it as a
// dedicated `if: always()` step, guaranteeing cleanup even when the apply test
// is cancelled or hits its timeout. Each stage is best-effort: a failure is
// reported but does not stop the remaining stages from being destroyed.
func TestRancherNodeDriverDestroy(t *testing.T) {
	rancherDir, terraformBin := e2eSetup(t)

	for i := len(terraformStages) - 1; i >= 0; i-- {
		stage := terraformStages[i]
		stageDir := filepath.Join(rancherDir, stage)
		extraEnv := stageEnv(stage)

		t.Run(stage, func(t *testing.T) {
			if err := terraform(t, terraformBin, stageDir, extraEnv, "init", "-input=false"); err != nil {
				t.Fatalf("init: %v", err)
			}
			if err := terraform(t, terraformBin, stageDir, extraEnv,
				"destroy", "-input=false", "-auto-approve"); err != nil {
				t.Errorf("destroy: %v", err)
			}
		})
	}
}

// e2eSetup validates the environment shared by the end-to-end tests and returns
// the showcase rancher directory and the terraform binary to use.
func e2eSetup(t *testing.T) (rancherDir, terraformBin string) {
	t.Helper()

	if os.Getenv("RANCHER_E2E") == "" {
		t.Skip("set RANCHER_E2E=1 to run the Rancher end-to-end test")
	}

	rancherDir = os.Getenv("RANCHER_E2E_SHOWCASE_DIR")
	if rancherDir == "" {
		t.Fatal("RANCHER_E2E_SHOWCASE_DIR must point at the cloned oxidecomputer/showcase/integration/rancher repository")
	}
	if _, err := os.Stat(rancherDir); err != nil {
		t.Fatalf("RANCHER_E2E_SHOWCASE_DIR %q: %v", rancherDir, err)
	}

	terraformBin = os.Getenv("RANCHER_E2E_TERRAFORM_BIN")
	if terraformBin == "" {
		terraformBin = "terraform"
	}
	if _, err := exec.LookPath(terraformBin); err != nil {
		t.Fatalf("terraform binary %q not found: %v", terraformBin, err)
	}

	return rancherDir, terraformBin
}

func terraform(t *testing.T, terraformBin string, dir string, extraEnv []string, args ...string) error {
	t.Helper()

	cmd := exec.Command(terraformBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"TF_IN_AUTOMATION=1",
		"TF_INPUT=0",
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	cmd.Stdout = t.Output()
	cmd.Stderr = t.Output()

	t.Logf("terraform %v (in %s)", args, dir)
	return cmd.Run()
}

func stageEnv(stage string) []string {
	prefix := "RANCHER_E2E_STAGE_" + strings.ToUpper(stage) + "_"

	var env []string
	for _, kv := range os.Environ() {
		if name, ok := strings.CutPrefix(kv, prefix); ok {
			env = append(env, name)
		}
	}
	return env
}
