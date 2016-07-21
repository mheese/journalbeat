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
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/publisher"
	"github.com/mheese/go-systemd/sdjournal"
)

var SeekPositions = map[string]bool{
	"cursor": true,
	"head":   true,
	"tail":   true,
}

var SeekFallbackPositions = map[string]bool{
	"none": true,
	"head": true,
	"tail": true,
}

// Journalbeat is the main Journalbeat struct
type Journalbeat struct {
	JbConfig             ConfigSettings
	writeCursorState     bool
	cursorStateFile      string
	cursorFlushSecs      int
	seekPosition         string
	cursorSeekFallback   string
	convertToNumbers     bool
	cleanFieldnames      bool
	moveMetadataLocation string
	defaultType          string

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

	if jb.JbConfig.Input.SeekPosition != nil {
		jb.seekPosition = *jb.JbConfig.Input.SeekPosition
	} else {
		jb.seekPosition = "tail"
	}

	if jb.JbConfig.Input.CursorSeekFallback != nil {
		jb.cursorSeekFallback = *jb.JbConfig.Input.CursorSeekFallback
	} else {
		jb.cursorSeekFallback = "tail"
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

	if jb.JbConfig.Input.DefaultType != nil {
		jb.defaultType = *jb.JbConfig.Input.DefaultType
	} else {
		jb.defaultType = "journal"
	}

	if _, ok := SeekPositions[jb.seekPosition]; !ok {
		errMsg := "seek_position must be either cursor, head, or tail"
		logp.Err(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	if _, ok := SeekFallbackPositions[jb.cursorSeekFallback]; !ok {
		errMsg := "cursor_seek_fallback must be either head, tail, or none"
		logp.Err(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

func (jb *Journalbeat) seekToPosition() error {
	position := jb.seekPosition
	// try seekToCursor first, if that is requested
	if position == "cursor" {
		cursor, err := ioutil.ReadFile(jb.cursorStateFile)
		if err != nil {
			logp.Warn("Could not seek to cursor: reading cursor state file failed: %v", err)
		} else {
			// try to seek to cursor and if successful return
			err = seekToHelper("cursor", jb.jr.Journal.SeekCursor(string(cursor)))
			if err == nil {
				return nil
			}
		}

		if jb.cursorSeekFallback == "none" {
			return err
		}

		position = jb.cursorSeekFallback
	}

	var err error
	switch position {
	case "head":
		err = seekToHelper("head", jb.jr.Journal.SeekHead())
	case "tail":
		err = seekToHelper("tail", jb.jr.Journal.SeekTail())
	}
	return err
}

func seekToHelper(position string, err error) error {
	if err == nil {
		logp.Info("Seek to " + position + " successful")
	} else {
		logp.Warn("Could not seek to %s: %v", position, err)
	}
	return err
}

// Setup prepares Journalbeat for the main loop (starts journalreader, etc.)
func (jb *Journalbeat) Setup(b *beat.Beat) error {
	logp.Info("Journalbeat Setup")
	jb.output = b.Publisher.Connect()
	// Buffer channel else write to it blocks when Stop is called while
	// FollowJournal waits to write next  event
	jb.done = make(chan int, 1)
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
	jb.output.Close()
	if jb.writeCursorState {
		jb.cursorChanFlush <- 1
	}
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
	// A little hack to get Followjournal to close correctly.
	// Write to buffered close channel and then read next event
	// else if Publish is stuck on a send it hangs
	jb.done <- 1
	select {
	case <-jb.recv:
	}
}

// Publish is used to publish read events to the beat output chain
func Publish(beat *beat.Beat, jb *Journalbeat) {
	logp.Info("Start sending events to output")
	for {
		ev := <-jb.recv

		// do some conversion, etc.
		m := MapStrFromJournalEntry(ev, jb.cleanFieldnames, jb.convertToNumbers)
		if jb.moveMetadataLocation != "" {
			m = MapStrMoveJournalMetadata(m, jb.moveMetadataLocation)
		}

		// add type if it does not exist yet (or if it is not a string)
		// TODO: type should be derived from the system journal
		_, ok := m["type"].(string)
		if !ok {
			m["type"] = jb.defaultType
		}

		// add input_type if it does not exist yet (or if it is not a string)
		// TODO: input_type should be derived from the system journal
		_, ok = m["input_type"].(string)
		if !ok {
			m["input_type"] = "journal"
		}

		// publish the event now
		//m := (common.MapStr)(ev)
		success := jb.output.PublishEvent(m, publisher.Sync, publisher.Guaranteed)
		// should never happen but if it does should definitely log an not save cursor
		if !success {
			logp.Err("PublishEvent returned false for cursor %s", ev["__CURSOR"])
			continue
		}

		// save cursor
		if jb.writeCursorState {
			cursor, ok := ev["__CURSOR"].(string)
			if ok {
				jb.cursorChan <- cursor
			}
		}
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
