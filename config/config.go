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

// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import (
	"fmt"
	"regexp"
	"time"
)

// Config provides the config settings for the journald reader
type Config struct {
	SeekPosition         string        `config:"seek_position"`
	ConvertToNumbers     bool          `config:"convert_to_numbers"`
	CleanFieldNames      bool          `config:"clean_field_names"`
	WriteCursorState     bool          `config:"write_cursor_state"`
	CursorStateFile      string        `config:"cursor_state_file"`
	CursorFlushPeriod    time.Duration `config:"cursor_flush_period"`
	CursorSeekFallback   string        `config:"cursor_seek_fallback"`
	MoveMetadataLocation string        `config:"move_metadata_to_field"`
	DefaultType          string        `config:"default_type"`
	Units                []string      `config:"units"`
}

// Named constants for the journal cursor placement positions
const (
	SeekPositionCursor  = "cursor"
	SeekPositionHead    = "head"
	SeekPositionTail    = "tail"
	SeekPositionDefault = "none"
)

var (
	seekPositions = map[string]struct{}{
		SeekPositionCursor: {},
		SeekPositionHead:   {},
		SeekPositionTail:   {},
	}

	seekFallbackPositions = map[string]struct{}{
		SeekPositionDefault: {},
		SeekPositionHead:    {},
		SeekPositionTail:    {},
	}

	// DefaultConfig is an instance of Config with default settings
	DefaultConfig = Config{
		SeekPosition:       SeekPositionTail,
		CursorStateFile:    ".journalbeat-cursor-state",
		CursorFlushPeriod:  5 * time.Second,
		CursorSeekFallback: SeekPositionTail,
		DefaultType:        "journal",
	}
)

// Validate turns Config into implementation of Validator and will be executed when Unpack is called
func (config *Config) Validate() error {
	// validate MoveMetadataLocation against the regexp. We don't want extra dots to appear
	validID := regexp.MustCompile(`\.{2,}|\.$`)
	if config.MoveMetadataLocation != "" && validID.MatchString(config.MoveMetadataLocation) {
		return fmt.Errorf("Wrong location for the Journal Metadata: %s", config.MoveMetadataLocation)
	}

	if _, ok := seekPositions[config.SeekPosition]; !ok {
		return fmt.Errorf("Invalid Seek Position: %v. Should be %s, %s or %s", config.SeekPosition, SeekPositionCursor, SeekPositionHead, SeekPositionTail)
	}

	if _, ok := seekFallbackPositions[config.CursorSeekFallback]; !ok {
		return fmt.Errorf("Invalid Cursor Seek Fallback Position: %v. Should be %s, %s or %s", config.SeekPosition, SeekPositionTail, SeekPositionHead, SeekPositionDefault)
	}
	return nil
}
