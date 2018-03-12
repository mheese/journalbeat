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

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

// eventSignal implements the op.Signaler interface
type eventSignal struct {
	ev        *eventReference
	completed chan<- *eventReference
}

// eventReference is used as a reference to the event being sent
type eventReference struct {
	cursor string
	body   common.MapStr
}

func (ref *eventSignal) Completed() {
	ref.completed <- ref.ev
}

func (ref *eventSignal) Failed() {
	logp.Warn("Failed to publish message with cursor %s", ref.ev.cursor)
}

func (ref *eventSignal) Canceled() {
	logp.Debug("pendingqueue", "Publishing message with cursor %s was canceled", ref.ev.cursor)
}

// managePendingQueueLoop runs the loop which manages the set of events waiting to be acked
func (jb *Journalbeat) managePendingQueueLoop() {
	jb.wg.Add(1)
	defer jb.wg.Done()
	pending := map[string]common.MapStr{}
	completed := map[string]common.MapStr{}
	queueChanged := false

	// diff returns the difference between this map and the other.
	diff := func(this, other map[string]common.MapStr) map[string]common.MapStr {
		result := map[string]common.MapStr{}
		for k, v := range this {
			if _, ok := other[k]; !ok {
				result[k] = v
			}
		}
		return result
	}

	// flush saves the map[string]common.MapStr to the JSON file on disk
	flush := func(source map[string]common.MapStr, dest string) error {
		tempFile, err := ioutil.TempFile(filepath.Dir(dest), fmt.Sprintf(".%s", filepath.Base(dest)))
		if err != nil {
			return err
		}

		if err = json.NewEncoder(tempFile).Encode(source); err != nil {
			_ = tempFile.Close()
			return err
		}

		_ = tempFile.Close()
		return os.Rename(tempFile.Name(), dest)
	}

	// on exit fully consume both queues and flush to disk the pending queue
	defer func() {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for evRef := range jb.pending {
				pending[evRef.cursor] = evRef.body
			}
		}()

		go func() {
			defer wg.Done()
			for evRef := range jb.completed {
				completed[evRef.cursor] = evRef.body
			}
		}()
		wg.Wait()

		logp.Info("Saving the pending queue, consists of %d messages", len(diff(pending, completed)))
		if err := flush(diff(pending, completed), jb.config.PendingQueue.File); err != nil {
			logp.Err("error writing pending queue %s: %s", jb.config.PendingQueue.File, err)
		}
	}()

	// flush the pending queue to disk periodically
	tick := time.Tick(jb.config.PendingQueue.FlushPeriod)
	for {
		select {
		case <-jb.done:
			return
		case p, ok := <-jb.pending:
			if ok {
				pending[p.cursor] = p.body
				queueChanged = true
			}
		case c, ok := <-jb.completed:
			if ok {
				completed[c.cursor] = c.body
				queueChanged = true
			}
		case <-tick:
			if !queueChanged {
				logp.Debug("pendingqueue", "Pending queue did not change")
				continue
			}
			result := diff(pending, completed)
			if err := flush(result, jb.config.PendingQueue.File); err != nil {
				logp.Err("error writing %s: %s", jb.config.PendingQueue.File, err)
			}
			pending = result
			queueChanged = false
			completed = map[string]common.MapStr{}
		}
	}
}

// writeCursorLoop runs the loop which flushes the current cursor position to a file
func (jb *Journalbeat) writeCursorLoop() {
	jb.wg.Add(1)
	defer jb.wg.Done()

	var cursor string
	saveCursorState := func(cursor string) {
		if cursor == "" {
			return
		}

		tempFile, err := ioutil.TempFile(filepath.Dir(jb.config.CursorStateFile), fmt.Sprintf(".%s", filepath.Base(jb.config.CursorStateFile)))
		if err != nil {
			logp.Err("Could not create cursor state file: %v", err)
			return
		}

		if _, err = tempFile.WriteString(cursor); err != nil {
			_ = tempFile.Close()
			logp.Err("Could not write to cursor state file: %v, cursor: %s", err, cursor)
			return
		}
		_ = tempFile.Close()
		if err := os.Rename(tempFile.Name(), jb.config.CursorStateFile); err != nil {
			logp.Err("Could not save cursor to the state file: %v, cursor: %s", err, cursor)
			return
		}
	}

	// save cursor for the last time when stop signal caught
	// Saving the cursor through defer guarantees that the jb.cursorChan has been fully consumed
	// and we are writing the cursor of the last message published.
	defer func() { saveCursorState(cursor) }()

	tick := time.Tick(jb.config.CursorFlushPeriod)

	for cursor = range jb.cursorChan {
		select {
		case <-tick:
			saveCursorState(cursor)
		default:
		}
	}
}
