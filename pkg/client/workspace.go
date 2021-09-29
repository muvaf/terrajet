/*
Copyright 2021 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/terrajet/pkg/json"
)

const (
	defaultAsyncTimeout = 1 * time.Hour
)

// todo: add logging.
// todo: print stdout during debug log.

type EnqueueFn func()

type Workspace struct {
	LastOperation *Operation
	Enqueue       EnqueueFn

	dir string
}

func (w *Workspace) ApplyAsync(_ context.Context) error {
	if w.LastOperation.EndTime == nil {
		return errors.Errorf("%s operation that started at %s is still running", w.LastOperation.Type, w.LastOperation.StartTime.String())
	}
	w.LastOperation.MarkStart("apply")
	ctx, cancel := context.WithDeadline(context.TODO(), w.LastOperation.StartTime.Add(defaultAsyncTimeout))
	go func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve", "-input=false", "-detailed-exitcode", "-json")
		cmd.Dir = w.dir
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			w.LastOperation.Err = errors.Wrapf(err, "cannot apply: %s", stderr.String())
		}
		w.LastOperation.MarkEnd()

		// After the operation is completed, we need to get the results saved on
		// the custom resource as soon as possible. We can wait for the next
		// reconciliation, enqueue manually or update the CR independent of the
		// reconciliation.
		w.Enqueue()
		cancel()
	}()
	return nil
}

type ApplyResult struct {
	State *json.StateV4
}

func (w *Workspace) Apply(ctx context.Context) (ApplyResult, error) {
	if w.LastOperation.EndTime == nil {
		return ApplyResult{}, errors.Errorf("%s operation that started at %s is still running", w.LastOperation.Type, w.LastOperation.StartTime.String())
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve", "-input=false", "-detailed-exitcode", "-json")
	cmd.Dir = w.dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return ApplyResult{}, errors.Wrapf(err, "cannot apply: %s", stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(w.dir, "terraform.tfstate"))
	if err != nil {
		return ApplyResult{}, errors.Wrap(err, "cannot read terraform state file")
	}
	s := &json.StateV4{}
	if err := json.JSParser.Unmarshal(raw, s); err != nil {
		return ApplyResult{}, errors.Wrap(err, "cannot unmarshal tfstate file")
	}
	return ApplyResult{State: s}, nil
}

func (w *Workspace) Destroy(_ context.Context) error {
	switch {
	// Destroy call is idempotent and can be called repeatedly.
	case w.LastOperation.Type == "destroy":
		return nil
	// We cannot run destroy until current non-destroy operation is completed.
	// TODO(muvaf): Gracefully terminate the ongoing apply operation?
	case w.LastOperation.Type != "destroy" && w.LastOperation.EndTime == nil:
		return errors.Errorf("%s operation that started at %s is still running", w.LastOperation.Type, w.LastOperation.StartTime.String())
	}
	w.LastOperation.MarkStart("destroy")
	ctx, cancel := context.WithDeadline(context.TODO(), w.LastOperation.StartTime.Add(defaultAsyncTimeout))
	go func() {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd := exec.CommandContext(ctx, "terraform", "destroy", "-auto-approve", "-input=false", "-detailed-exitcode", "-json")
		cmd.Dir = w.dir
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			w.LastOperation.Err = errors.Wrapf(err, "cannot destroy: %s", stderr.String())
		}
		w.LastOperation.MarkEnd()

		// After the operation is completed, we need to get the results saved on
		// the custom resource as soon as possible. We can wait for the next
		// reconcilitaion, enqueue manually or update the CR independent of the
		// reconciliation.
		w.Enqueue()
		cancel()
	}()
	return nil
}

type RefreshResult struct {
	IsApplying         bool
	IsDestroying       bool
	State              *json.StateV4
	LastOperationError error
}

func (w *Workspace) Refresh(ctx context.Context) (RefreshResult, error) {
	if w.LastOperation.StartTime != nil {
		// The last operation is still ongoing.
		if w.LastOperation.EndTime == nil {
			return RefreshResult{
				IsApplying:   w.LastOperation.Type == "apply",
				IsDestroying: w.LastOperation.Type == "destroy",
			}, nil
		}
		// We know that the operation finished, so we need to flush so that new
		// operation can be started.
		defer w.LastOperation.Flush()

		// The last operation finished with error.
		if w.LastOperation.Err != nil {
			return RefreshResult{
				IsApplying:         w.LastOperation.Type == "apply",
				IsDestroying:       w.LastOperation.Type == "destroy",
				LastOperationError: errors.Wrapf(w.LastOperation.Err, "%s operation failed", w.LastOperation.Type),
			}, nil
		}
		// The deletion is completed so there is no resource to refresh.
		if w.LastOperation.Type == "destroy" {
			return RefreshResult{}, kerrors.NewNotFound(schema.GroupResource{}, "")
		}
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "terraform", "apply", "-refresh-only", "-auto-approve", "-input=false", "-detailed-exitcode", "-json")
	cmd.Dir = w.dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		// todo: handle the case where resource is not found.
		return RefreshResult{}, errors.Wrapf(err, "cannot refresh: %s", stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(w.dir, "terraform.tfstate"))
	if err != nil {
		return RefreshResult{}, errors.Wrap(err, "cannot read terraform state file")
	}
	s := &json.StateV4{}
	if err := json.JSParser.Unmarshal(raw, s); err != nil {
		return RefreshResult{}, errors.Wrap(err, "cannot unmarshal tfstate file")
	}
	return RefreshResult{State: s}, nil
}

type PlanResult struct {
	Exists   bool
	UpToDate bool
}

func (w *Workspace) Plan(ctx context.Context) (PlanResult, error) {
	// The last operation is still ongoing.
	if w.LastOperation.StartTime != nil && w.LastOperation.EndTime == nil {
		return PlanResult{}, errors.Errorf("%s operation that started at %s is still running", w.LastOperation.Type, w.LastOperation.StartTime.String())
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "terraform", "plan", "-refresh=false", "-input=false", "-detailed-exitcode", "-json")
	cmd.Dir = w.dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return PlanResult{}, errors.Wrapf(err, "cannot plan: %s", stderr.String())
	}
	line := ""
	for _, l := range strings.Split(stdout.String(), "\n") {
		if strings.Contains(l, `"type":"change_summary"`) {
			line = l
			break
		}
	}
	if line == "" {
		return PlanResult{}, errors.Errorf("cannot find the change summary line in plan log: %s", stdout.String())
	}
	type plan struct {
		Changes struct {
			Add    float64 `json:"add,omitempty"`
			Change float64 `json:"change,omitempty"`
		} `json:"changes,omitempty"`
	}
	p := &plan{}
	if err := json.JSParser.Unmarshal([]byte(line), p); err != nil {
		return PlanResult{}, errors.Wrap(err, "cannot unmarshal change summary json")
	}
	return PlanResult{
		Exists:   p.Changes.Add == 0,
		UpToDate: p.Changes.Change == 0,
	}, nil
}
