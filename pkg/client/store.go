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
	"os"
	"path/filepath"
	"sync"

	xpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"

	"github.com/crossplane-contrib/terrajet/pkg/json"
	"github.com/crossplane-contrib/terrajet/pkg/terraform/resource"
	"github.com/crossplane-contrib/terrajet/pkg/tfcli"
)

func NewWorkspaceStore(setup tfcli.TerraformSetup) *WorkspaceStore {
	return &WorkspaceStore{setup: setup}
}

type WorkspaceStore struct {
	// store holds information about ongoing operations of given resource.
	// Since there can be multiple calls that add/remove values from the map at
	// the same time, it has to be safe for concurrency since those operations
	// cause rehashing in some cases.
	store sync.Map

	setup tfcli.TerraformSetup
}

// TODO(muvaf): Take EnqueueFn as parameter tow WorkspaceStore?

func (ws *WorkspaceStore) Workspace(tr resource.Terraformed, enq EnqueueFn) (*Workspace, error) {
	dir := filepath.Join(os.TempDir(), string(tr.GetUID()))
	fp, err := NewFileProducer(tr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create a new file producer")
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, errors.Wrap(err, "cannot create directory for workspace")
	}
	_, err = os.Stat(filepath.Join(dir, "terraform.tfstate"))
	if xpresource.Ignore(os.IsNotExist, err) != nil {
		return nil, errors.Wrap(err, "cannot state terraform.tfstate file")
	}
	// todo: If there is no open operation, delete terraform lock file.
	if os.IsNotExist(err) {
		s, err := fp.TFState()
		if err != nil {
			return nil, errors.Wrap(err, "cannot produce tfstate")
		}
		rawState, err := json.JSParser.Marshal(s)
		if err != nil {
			return nil, errors.Wrap(err, "cannot marshal state object")
		}
		if err := os.WriteFile(filepath.Join(dir, "terraform.tfstate"), rawState, os.ModePerm); err != nil {
			return nil, errors.Wrap(err, "cannot write tfstate file")
		}
	}
	rawHCL, err := json.JSParser.Marshal(fp.MainTF())
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal main hcl object")
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf.json"), rawHCL, os.ModePerm); err != nil {
		return nil, errors.Wrap(err, "cannot write tfstate file")
	}
	w, _ := ws.store.LoadOrStore(tr.GetUID(), &Workspace{
		Enqueue: enq,
		dir:     dir,
	})
	return w.(*Workspace), nil
}

func (ws *WorkspaceStore) Remove(obj xpresource.Object) error {
	w, ok := ws.store.Load(obj.GetUID())
	if !ok {
		return nil
	}
	if err := os.RemoveAll(w.(*Workspace).dir); err != nil {
		return errors.Wrap(err, "cannot remove workspace folder")
	}
	ws.store.Delete(obj.GetUID())
	return nil
}
