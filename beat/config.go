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
	"github.com/elastic/beats/libbeat/common"
)

// JournalReaderConfig provides the config settings for the journald reader
type JournalReaderConfig struct {
	WriteCursorState     *bool          `config:"write_cursor_state"`
	CursorStateFile      *string        `config:"cursor_state_file"`
	FlushCursorSecs      *int           `config:"flush_cursor_secs"`
	SeekPosition         *string        `config:"seek_position"`
	CursorSeekFallback   *string        `config:"cursor_seek_fallback"`
	ConvertToNumbers     *bool          `config:"convert_to_numbers"`
	CleanFieldNames      *bool          `config:"clean_field_names"`
	MoveMetadataLocation *string        `config:"move_metadata_to_field"`
	FieldsDest           *string        `config:"fields_dest"`
	Fields               *common.MapStr `config:"fields"`
}

// ConfigSettings holds JournalConfig at the Input section of the config file
type ConfigSettings struct {
	Input JournalReaderConfig `config:"input"`
}
