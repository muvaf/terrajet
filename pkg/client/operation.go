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

import "time"

type Operation struct {
	Type      string
	StartTime *time.Time
	EndTime   *time.Time

	err error
}

func (o Operation) MarkStart(t string) {
	o.Type = t
	now := time.Now()
	o.StartTime = &now
	o.EndTime = nil
	o.err = nil
}

func (o Operation) MarkEnd() {
	now := time.Now()
	o.EndTime = &now
}

func (o Operation) Flush() {
	o.Type = ""
	o.StartTime = nil
	o.EndTime = nil
	o.err = nil
}
