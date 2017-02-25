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

package beater

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/publisher"
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
	logp.Debug("Publishing message with cursor %s was canceled", ref.ev.cursor)
}

// managePendingQueueLoop runs the loop which manages the set of events waiting to be acked
func (jb *Journalbeat) managePendingQueueLoop() {
	jb.wg.Add(1)
	defer jb.wg.Done()
	pending := map[string]common.MapStr{}
	completed := map[string]common.MapStr{}

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
		file, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer file.Close()

		return json.NewEncoder(file).Encode(source)
	}

	// load loads the map[string]common.MapStr from the JSON file on disk
	load := func(source string, dest *map[string]common.MapStr) error {
		file, err := os.Open(source)
		if err != nil {
			return err
		}
		defer file.Close()

		return json.NewDecoder(file).Decode(dest)
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

	// load the previously saved queue of unsent events and try to publish them if any
	if err := load(jb.config.PendingQueue.File, &pending); err != nil {
		logp.Warn("could not read the pending queue: %s", err)
	}
	logp.Info("Loaded %d events, trying to publish", len(pending))
	for cursor, event := range pending {
		jb.client.PublishEvent(event, publisher.Signal(&eventSignal{&eventReference{cursor, event}, jb.completed}), publisher.Guaranteed)
	}

	// flush the pending queue to disk periodically
	tick := time.Tick(jb.config.PendingQueue.FlushPeriod)
	for {
		select {
		case <-jb.done:
			return
		case p := <-jb.pending:
			pending[p.cursor] = p.body
		case c := <-jb.completed:
			completed[c.cursor] = c.body
		case <-tick:
			result := diff(pending, completed)
			if err := flush(result, jb.config.PendingQueue.File); err != nil {
				logp.Err("error writing %s: %s", jb.config.PendingQueue.File, err)
			}
			pending = result
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
		if cursor != "" {
			if err := ioutil.WriteFile(jb.config.CursorStateFile, []byte(cursor), 0644); err != nil {
				logp.Err("Could not write to cursor state file: %v", err)
			}
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
