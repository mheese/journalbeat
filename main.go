// Copyright 2016 Marcus Heese
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

package main

import (
	"github.com/elastic/beats/libbeat/beat"
	journalbeat "github.com/mheese/journalbeat/beat"
)

// Name is the name of the beat
const Name = "journalbeat"
const Version = "0.1.0"

func main() {
	beat.Run(Name, Version, journalbeat.New())
}
