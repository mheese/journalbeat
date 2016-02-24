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

package beat

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/cfgfile"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/publisher"
	"github.com/mheese/go-systemd/sdjournal"
)

// Journalbeat is the main Journalbeat struct
type Journalbeat struct {
	JbConfig             ConfigSettings
	writeCursorState     bool
	cursorStateFile      string
	cursorFlushSecs      int
	seekToCursor         bool
	seekToHead           bool
	seekToTail           bool
	convertToNumbers     bool
	cleanFieldnames      bool
	moveMetadataLocation string
	fields               common.MapStr

	jr   *sdjournal.JournalReader
	done chan int
	recv chan sdjournal.JournalEntry

	cursorChan      chan string
	cursorChanFlush chan int

	output publisher.Client
}

// New creates a new Journalbeat object and returns. Should be done once in main
func New() *Journalbeat {
	logp.Info("New Journalbeat")
	return &Journalbeat{}
}

// Config parses configuration data and prepares for Setup
func (jb *Journalbeat) Config(b *beat.Beat) error {
	logp.Info("Journalbeat Config")
	err := cfgfile.Read(&jb.JbConfig, "")
	if err != nil {
		logp.Err("Error reading configuration file: %v", err)
		return err
	}

	if jb.JbConfig.Input.WriteCursorState != nil {
		jb.writeCursorState = *jb.JbConfig.Input.WriteCursorState
	} else {
		jb.writeCursorState = false
	}

	if jb.JbConfig.Input.CursorStateFile != nil {
		jb.cursorStateFile = *jb.JbConfig.Input.CursorStateFile
	} else {
		jb.cursorStateFile = ".journalbeat-cursor-state"
	}

	if jb.JbConfig.Input.FlushCursorSecs != nil {
		jb.cursorFlushSecs = *jb.JbConfig.Input.FlushCursorSecs
	} else {
		jb.cursorFlushSecs = 5
	}

	if jb.JbConfig.Input.SeekToCursor != nil {
		jb.seekToCursor = *jb.JbConfig.Input.SeekToCursor
	} else {
		jb.seekToCursor = false
	}

	if jb.JbConfig.Input.SeekToHead != nil {
		jb.seekToHead = *jb.JbConfig.Input.SeekToHead
	} else {
		jb.seekToHead = false
	}

	if jb.JbConfig.Input.SeekToTail != nil {
		jb.seekToTail = *jb.JbConfig.Input.SeekToTail
	} else {
		jb.seekToTail = true
	}

	if jb.JbConfig.Input.ConvertToNumbers != nil {
		jb.convertToNumbers = *jb.JbConfig.Input.ConvertToNumbers
	} else {
		jb.convertToNumbers = false
	}

	if jb.JbConfig.Input.CleanFieldNames != nil {
		jb.cleanFieldnames = *jb.JbConfig.Input.CleanFieldNames
	} else {
		jb.cleanFieldnames = false
	}

	if jb.JbConfig.Input.MoveMetadataLocation != nil {
		jb.moveMetadataLocation = *jb.JbConfig.Input.MoveMetadataLocation
	} else {
		jb.moveMetadataLocation = ""
	}

	if jb.JbConfig.Input.Fields != nil {
		jb.fields = *jb.JbConfig.Input.Fields
	}

	if !jb.seekToCursor && !jb.seekToHead && !jb.seekToTail {
		errMsg := "Either seek_to_cursor, seek_to_head or seek_to_tail need to be true"
		logp.Err(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

func (jb *Journalbeat) seekToPosition() error {
	var err error
	// try seekToCursor first, if that is requested
	if jb.seekToCursor {
		cursor, err2 := ioutil.ReadFile(jb.cursorStateFile)
		if err != nil {
			err = fmt.Errorf("reading cursor state file failed: %v", err2)
			logp.Err("Could not seek to cursor: %v", err)
		} else {
			// try to seek to cursor
			err2 = jb.jr.Journal.SeekCursor(string(cursor))
			if err2 != nil {
				err = fmt.Errorf("seek to cursor failed: %v", err2)
				logp.Err("Could not seek to cursor: %v", err)
			} else {
				// seeking successful, return
				logp.Info("Seek to cursor successful")
				return nil
			}
		}
	}

	if jb.seekToHead {
		err2 := jb.jr.Journal.SeekHead()
		if err2 != nil {
			err = fmt.Errorf("seek to head failed: %v", err2)
			logp.Err("Could not seek to head: %v", err)
		} else {
			// seeking successful, return
			logp.Info("Seek to head successful")
			return nil
		}
	}

	if jb.seekToTail {
		err2 := jb.jr.Journal.SeekTail()
		if err2 != nil {
			err = fmt.Errorf("seek to tail failed: %v", err2)
			logp.Err("Could not seek to tail: %v", err)
		} else {
			// seeking successful, return
			logp.Info("Seek to tail successful")
			return nil
		}
	}

	return err
}

// Setup prepares Journalbeat for the main loop (starts journalreader, etc.)
func (jb *Journalbeat) Setup(b *beat.Beat) error {
	logp.Info("Journalbeat Setup")
	jb.output = b.Events
	jb.done = make(chan int)
	jb.recv = make(chan sdjournal.JournalEntry)
	jb.cursorChan = make(chan string)
	jb.cursorChanFlush = make(chan int)

	jr, err := sdjournal.NewJournalReader(sdjournal.JournalReaderConfig{
		Since: time.Duration(1),
		//          NumFromTail: 0,
	})
	if err != nil {
		logp.Err("Could not create JournalReader")
		return err
	}

	jb.jr = jr

	// seek to position
	err = jb.seekToPosition()
	if err != nil {
		errMsg := fmt.Sprintf("seeking to a good position in journal failed: %v", err)
		logp.Err(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// done with setup
	return nil
}

// Cleanup cleans up resources
func (jb *Journalbeat) Cleanup(b *beat.Beat) error {
	logp.Info("Journalbeat Cleanup")
	jb.jr.Close()
	jb.cursorChanFlush <- 1
	close(jb.done)
	close(jb.recv)
	close(jb.cursorChan)
	close(jb.cursorChanFlush)
	return nil
}

// Run is the main event loop: read from journald and pass it to Publish
func (jb *Journalbeat) Run(b *beat.Beat) error {
	logp.Info("Journalbeat Run")

	// if requested, start the WriteCursorLoop
	if jb.writeCursorState {
		go WriteCursorLoop(jb)
	}

	// Publishes event to output
	go Publish(b, jb)

	// Blocks progressing
	jb.jr.FollowJournal(jb.done, jb.recv)
	return nil
}

// Stop stops the journalbeat
func (jb *Journalbeat) Stop() {
	logp.Info("Journalbeat Stop")
	jb.done <- 1
}

// Publish is used to publish read events to the beat output chain
func Publish(beat *beat.Beat, jb *Journalbeat) {
	logp.Info("Start sending events to output")
	for {
		ev := <-jb.recv

		// save cursor
		if jb.writeCursorState {
			cursor, ok := ev["__CURSOR"].(string)
			if ok {
				jb.cursorChan <- cursor
			}
		}

		// do some conversion, etc.
		m := MapStrFromJournalEntry(ev, jb.cleanFieldnames, jb.convertToNumbers)
		if jb.moveMetadataLocation != "" {
			m = MapStrMoveJournalMetadata(m, jb.moveMetadataLocation)
		}

		// add arbitrary fields.
		if jb.fields != nil {
			m["fields"] = jb.fields
		}

		// publish the event now
		//m := (common.MapStr)(ev)
		jb.output.PublishEvent(m)
	}
}

// WriteCursorLoop runs the loop which flushes the current cursor position to
// a file
func WriteCursorLoop(jb *Journalbeat) {
	var cursor, oldCursor string
	before := time.Now()
	stop := false
	for {
		// select next event
		select {
		case <-jb.cursorChanFlush:
			stop = true
		case c := <-jb.cursorChan:
			cursor = c
		case <-time.After(time.Duration(jb.cursorFlushSecs) * time.Second):
		}

		// stop immediately if we are supposed to
		if stop {
			break
		}

		// check if we need to flush
		now := time.Now()
		if now.Sub(before) > time.Duration(jb.cursorFlushSecs)*time.Second {
			before = now
			if cursor != oldCursor {
				jb.saveCursorState(cursor)
				oldCursor = cursor
			}
		}
	}

	logp.Info("flushing cursor state for the last time")
	jb.saveCursorState(cursor)
}

func (jb *Journalbeat) saveCursorState(cursor string) {
	if cursor != "" {
		err := ioutil.WriteFile(jb.cursorStateFile, []byte(cursor), 0644)
		if err != nil {
			logp.Err("Could not write to cursor state file: %v", err)
		}
	}
}
