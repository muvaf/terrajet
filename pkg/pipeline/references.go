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

package pipeline

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/crossplane-contrib/terrajet/pkg/pipeline/templates"
	"github.com/muvaf/typewriter/pkg/wrapper"

	tjtypes "github.com/crossplane-contrib/terrajet/pkg/types"
)

func NewReferenceGenerator(pkg *types.Package, localDirectoryPath string) *ReferenceGenerator {
	return &ReferenceGenerator{
		LocalDirectoryPath: localDirectoryPath,
		pkg:                pkg,
	}
}

type ReferenceGenerator struct {
	LocalDirectoryPath string

	pkg *types.Package
}

func (rg *ReferenceGenerator) Generate(kind string, refs []tjtypes.Reference) error {
	if len(refs) == 0 {
		return nil
	}
	file := wrapper.NewFile(rg.pkg.Path(), rg.pkg.Name(), templates.CRDReferencesTemplate,
		wrapper.WithGenStatement(GenStatement),
		wrapper.WithHeaderPath("hack/boilerplate.go.txt"), // todo
		// wrapper.LinterEnabled(),
	)

	extractor := file.Imports.UsePackage("github.com/crossplane/crossplane-runtime/pkg/reference") + "ExternalName()"
	refList := make([]ReferenceToPrint, len(refs))
	for i, ref := range refs {
		refList[i] = ReferenceToPrint{
			Reference:  ref,
			RemoteType: file.Imports.UseType(ref.RemoteTypePath),
			Extractor:  extractor,
		}
		if ref.ExtractorPath != nil {
			refList[i].Extractor = file.Imports.UseType(*ref.ExtractorPath)
		}
	}
	vars := map[string]interface{}{
		"CRD": map[string]string{
			"Kind": kind,
		},
		"References":        refList,
		"ReferencePkgAlias": file.Imports.UsePackage("github.com/crossplane/crossplane-runtime/pkg/reference"),
	}
	filePath := filepath.Join(rg.LocalDirectoryPath, fmt.Sprintf("zz_%s_references.go", strings.ToLower(kind)))
	return errors.Wrap(file.Write(filePath, vars, os.ModePerm), "cannot write references file")
}

type ReferenceToPrint struct {
	tjtypes.Reference

	RemoteType string
	Extractor  string
}
