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

package types

import (
	"go/token"
	"go/types"
)

var (
	ReferenceType = types.NewPointer(
		types.NewNamed(
			types.NewTypeName(
				token.NoPos,
				types.NewPackage("github.com/crossplane/crossplane-runtime/apis/common/v1", "v1beta1"), "Reference", nil), nil, nil))
	ReferenceListType = types.NewSlice(
		types.NewNamed(
			types.NewTypeName(
				token.NoPos,
				types.NewPackage("github.com/crossplane/crossplane-runtime/apis/common/v1", "v1beta1"), "Reference", nil), nil, nil))
	SelectorType = types.NewPointer(
		types.NewNamed(
			types.NewTypeName(
				token.NoPos,
				types.NewPackage("github.com/crossplane/crossplane-runtime/apis/common/v1", "v1beta1"), "Selector", nil), nil, nil))
)

type Reference struct {
	ReferenceInput
	GoFieldPath string
	IsPointer   bool
	IsList      bool
}

type ReferenceInput struct {
	// RemoteTypePath is the Go package path of the type that will be the source
	// of information. The format is <package path>.<type name>
	// Example: github.com/crossplane-contrib/provider-tf-aws/apis/vpc/v1alpha1.VPC
	RemoteTypePath string

	// ExtractorPath is the Go package path to the extractor that will take the remote
	// type and produce the value to be stored in the local field. If empty,
	// external name annotation will be used. The format is <package path>.<func name>
	// Example: github.com/crossplane-contrib/provider-tf-aws/apis/iam/v1alpha1.IAMRoleARN()
	ExtractorPath *string

	TypeName  string
	FieldName string
}
