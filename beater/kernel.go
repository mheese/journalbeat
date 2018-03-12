// Copyright (c) 2018 Marcus Heese
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package beater

import "fmt"

func (jb *Journalbeat) addKernel() error {
	if len(jb.config.Units) > 0 && jb.config.Kernel {
		err := jb.addMatchesForKernel()
		if err != nil {
			return fmt.Errorf("Adding filter for kernel failed: %v", err)
		}
	}
	return nil
}

func (jb *Journalbeat) addMatchesForKernel() error {
	err := jb.journal.AddMatch("_TRANSPORT=kernel")
	if err != nil {
		return err
	}
	return jb.journal.AddDisjunction()
}
