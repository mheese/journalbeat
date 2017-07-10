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
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/publisher"
	"github.com/mheese/journalbeat/config"
	"github.com/mheese/journalbeat/journal"
)

// Journalbeat is the main Journalbeat struct
type Journalbeat struct {
	done   chan struct{}
	config config.Config
	client publisher.Client

	journal *sdjournal.Journal

	cursorChan         chan string
	pending, completed chan *eventReference
	wg                 sync.WaitGroup
}

func (jb *Journalbeat) initJournal() error {
	var err error

	seekToHelper := func(position string, err error) error {
		if err == nil {
			logp.Info("Seek to %s successful", position)
		} else {
			logp.Warn("Could not seek to %s: %v", position, err)
		}
		return err
	}

	// connect to the Systemd Journal
	switch len(jb.config.JournalPaths) {
	case 0:
		if jb.journal, err = sdjournal.NewJournal(); err != nil {
			return err
		}
	case 1:
		fi, err := os.Stat(jb.config.JournalPaths[0])
		if err != nil {
			return err
		}
		if fi.IsDir() {
			if jb.journal, err = sdjournal.NewJournalFromDir(jb.config.JournalPaths[0]); err != nil {
				return err
			}
		} else {
			if jb.journal, err = sdjournal.NewJournalFromFiles(jb.config.JournalPaths...); err != nil {
				return err
			}
		}
	default:
		if jb.journal, err = sdjournal.NewJournalFromFiles(jb.config.JournalPaths...); err != nil {
			return err
		}
	}

	// add specific units to monitor if any
	for _, unit := range jb.config.Units {
		if err = jb.journal.AddMatch(sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT + "=" + unit); err != nil {
			return fmt.Errorf("Filtering unit %s failed: %v", unit, err)
		}
	}

	// seek position
	position := jb.config.SeekPosition
	// try seekToCursor first, if that is requested
	if position == config.SeekPositionCursor {
		if cursor, err := ioutil.ReadFile(jb.config.CursorStateFile); err != nil {
			logp.Warn("Could not seek to cursor: reading cursor state file failed: %v", err)
		} else {
			// try to seek to cursor and if successful return
			if err = seekToHelper(config.SeekPositionCursor, jb.journal.SeekCursor(string(cursor))); err == nil {
				return nil
			}
		}

		if jb.config.CursorSeekFallback == config.SeekPositionDefault {
			return err
		}

		position = jb.config.CursorSeekFallback
	}

	switch position {
	case config.SeekPositionHead:
		err = seekToHelper(config.SeekPositionHead, jb.journal.SeekHead())
	case config.SeekPositionTail:
		err = seekToHelper(config.SeekPositionTail, jb.journal.SeekTail())
	}

	if err != nil {
		return fmt.Errorf("Seeking to a good position in journal failed: %v", err)
	}

	return nil
}

func (jb *Journalbeat) publishPending() error {
	pending := map[string]common.MapStr{}
	file, err := os.Open(jb.config.PendingQueue.File)
	if err != nil {
		return err
	}
	defer file.Close()

	if err = json.NewDecoder(file).Decode(&pending); err != nil {
		return err
	}

	logp.Info("Loaded %d events, trying to publish", len(pending))
	for cursor, event := range pending {
		jb.client.PublishEvent(event, publisher.Signal(&eventSignal{&eventReference{cursor, event}, jb.completed}), publisher.Guaranteed)
	}

	return nil
}

// New creates beater
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	config := config.DefaultConfig
	var err error
	if err = cfg.Unpack(&config); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	jb := &Journalbeat{
		config:     config,
		done:       make(chan struct{}),
		cursorChan: make(chan string),
		pending:    make(chan *eventReference),
		completed:  make(chan *eventReference),
	}

	if err = jb.initJournal(); err != nil {
		logp.Err("Failed to connect to the Systemd Journal: %v", err)
		return nil, err
	}

	return jb, nil
}

// Run is the main event loop: read from journald and pass it to Publish
func (jb *Journalbeat) Run(b *beat.Beat) error {
	logp.Info("Journalbeat is running!")
	defer func() {
		_ = jb.journal.Close()
		close(jb.cursorChan)
		close(jb.pending)
		close(jb.completed)
		jb.wg.Wait()
	}()

	jb.client = b.Publisher.Connect()

	go jb.managePendingQueueLoop()

	if jb.config.WriteCursorState {
		go jb.writeCursorLoop()
	}

	// load the previously saved queue of unsent events and try to publish them if any
	if err := jb.publishPending(); err != nil {
		logp.Warn("could not read the pending queue: %s", err)
	}

	for rawEvent := range journal.Follow(jb.journal, jb.done) {
		//convert sdjournal.JournalEntry to common.MapStr
		event := MapStrFromJournalEntry(
			rawEvent,
			jb.config.CleanFieldNames,
			jb.config.ConvertToNumbers,
			jb.config.MoveMetadataLocation)

		if _, ok := event["type"].(string); !ok {
			event["type"] = jb.config.DefaultType
		}
		event["@timestamp"] = common.Time(time.Unix(0, int64(rawEvent.RealtimeTimestamp)*1000))
		// add _REALTIME_TIMESTAMP until https://github.com/elastic/elasticsearch/issues/12829 is closed
		event["@realtime_timestamp"] = int64(rawEvent.RealtimeTimestamp)

		ref := &eventReference{rawEvent.Cursor, event}
		if jb.client.PublishEvent(event, publisher.Signal(&eventSignal{ref, jb.completed}), publisher.Guaranteed) {
			jb.pending <- ref

			// save cursor
			if jb.config.WriteCursorState {
				jb.cursorChan <- rawEvent.Cursor
			}
		}
	}
	return nil
}

// Stop stops Journalbeat execution
func (jb *Journalbeat) Stop() {
	logp.Info("Stopping Journalbeat")
	close(jb.done)
	_ = jb.client.Close()
}
