// Copyright 2017 Marcus Heese
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

package journal

import (
	"io"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/elastic/beats/libbeat/logp"
)

// SD_JOURNAL_FIELD_CATALOG_ENTRY stores the name of the JournalEntry field to export Catalog entry to.
const SD_JOURNAL_FIELD_CATALOG_ENTRY = "CATALOG_ENTRY"

// Follow follows the journald and writes the entries to the output channel
// It is a slightly reworked version of sdjournal.Follow to fit our needs.
func Follow(journal *sdjournal.Journal, stop <-chan struct{}) <-chan *sdjournal.JournalEntry {
	readEntry := func(journal *sdjournal.Journal) (*sdjournal.JournalEntry, error) {
		c, err := journal.Next()
		if err != nil {
			return nil, err
		}

		if c == 0 {
			return nil, io.EOF
		}

		entry, err := journal.GetEntry()
		if err != nil {
			return nil, err
		}
		return entry, nil
	}

	out := make(chan *sdjournal.JournalEntry)

	go func(journal *sdjournal.Journal, stop <-chan struct{}, out chan<- *sdjournal.JournalEntry) {
		defer close(out)
		eventWaitCh := make(chan int)

	process:
		for {
			entry, err := readEntry(journal)
			if err != nil && err != io.EOF {
				logp.Err("Received unknown error when reading a new entry: %v", err)
				return
			}
			if entry != nil {
				if _, ok := entry.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE_ID]; ok {
					if catalogEntry, err := journal.GetCatalog(); err == nil {
						entry.Fields[SD_JOURNAL_FIELD_CATALOG_ENTRY] = catalogEntry
					}
				}
				// non-blocking return
				select {
				case <-stop:
					return
				case out <- entry:
					continue process
				}
			}

			// We're at the tail, so wait for new events or time out.
			// Holds journal events to process. Tightly bounded for now unless there's a
			// reason to unblock the journal watch routine more quickly.
			for {
				go func() {
					select {
					case <-stop:
					case eventWaitCh <- journal.Wait(100 * time.Millisecond):
					}
				}()

				select {
				case <-stop:
					return
				case e := <-eventWaitCh:
					switch e {
					case sdjournal.SD_JOURNAL_NOP:
						// the journal did not change since the last invocation
					case sdjournal.SD_JOURNAL_APPEND, sdjournal.SD_JOURNAL_INVALIDATE:
						continue process
					default:
						logp.Err("Received unknown event: %d", e)
					}
				}
			}
		}
	}(journal, stop, out)

	return out
}
