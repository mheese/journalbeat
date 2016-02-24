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
	"strconv"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/common"
	"github.com/mheese/go-systemd/sdjournal"
)

// MapStrFromJournalEntry takes a JournalD entry and converts it to an event
// that is more compatible with the Elasitc products. It will perform the
// following additional steps to an event:
// - add "@timestamp" field from either _SOURCE_REALTIME_TIMESTAMP,
//   __REALTIME_TIMESTAMP or time.Now() (in that order depending on success)
// - lowercase all fields (seriously, who wants to type caps all day?!?)
// - remove underscores from the beginning of fields as they are reserved in
//   ElasticSearch for metadata information
// - fields that can be converted to numbers, will be converted to numbers
func MapStrFromJournalEntry(ev sdjournal.JournalEntry, cleanKeys bool, convertToNumbers bool) common.MapStr {
	// sdjournal.JournalEntry and common.MapStr are the same type alias:
	// map[string]interface{}
	old := (common.MapStr)(ev)
	m := make(common.MapStr)

	// range over the old map and create a new one
	for k, v := range old {
		nk := makeNewKey(k, cleanKeys)
		nv := makeNewValue(v, convertToNumbers)
		m[nk] = nv
	}

	// ensure the "@timestamp" field
	timestampFromJournalEntry := func() time.Time {
		srt, ok := old["_SOURCE_REALTIME_TIMESTAMP"].(int64)
		if ok {
			// srt is in microseconds epoch
			return time.Unix(0, srt*1000)
		}

		rt, ok := old["__REALTIME_TIMESTAMP"].(int64)
		if ok {
			// rt is in microseconds epoch
			return time.Unix(0, rt*1000)
		}

		// nothing worked ... get the last resort
		return time.Now()
	}
	m.EnsureTimestampField(timestampFromJournalEntry)

	// return with the new map
	return m
}

// MapStrMoveJournalMetadata will create a separate map for the Journal
// metadata and move all fields except "message" and "@timestamp" to the
// separate map which will be available at key "location"
func MapStrMoveJournalMetadata(in common.MapStr, location string) common.MapStr {
	if location == "" {
		return in
	}

	ts, okTs := in["@timestamp"]
	msg, okMsg := in["message"]

	if okTs {
		delete(in, "@timestamp")
	}

	if okMsg {
		delete(in, "message")
	}

	// now create a new map, fill it and return it
	m := make(common.MapStr)
	if okTs {
		m["@timestamp"] = ts
	}
	if okMsg {
		m["message"] = msg
	}

	// move nested like with fields
	mm := MapStrMoveMapToLocation(in, location)
	for k, v := range mm {
		m[k] = v
	}

	return m
}

func MapStrMoveMapToLocation(in common.MapStr, location string) common.MapStr {
	// nothing to move
	if location == "" {
		return in
	}

	m := make(common.MapStr)
	dests := strings.Split(location, ".")

	// oh man ... this would be so easy in a recursive function
	// welcome to stupid optimized iterative style ...
	var newM common.MapStr
	for i := 0; i < len(dests); i++ {
		if i == 0 {
			newM = m
		}

		if i+1 == len(dests) {
			n := dests[i]
			newM[n] = in
			break
		}

		tmp := make(common.MapStr)
		n := dests[i]
		newM[n] = tmp
		newM = tmp

	}

	return m
}

func makeNewKey(key string, cleanKeys bool) string {
	if !cleanKeys {
		return key
	}

	k := strings.ToLower(key)
	return strings.TrimLeft(k, "_")
}

func makeNewValue(value interface{}, convertToNumbers bool) interface{} {
	if !convertToNumbers {
		return value
	}

	switch v := value.(type) {
	// for now I can only think of the string scenario that is interesting here
	case string:
		// convert booleans if possible
		// strconv.ParseBool is unfortunately too forgiving, we only want the hard
		// words
		if v == "true" || v == "TRUE" || v == "True" {
			return true
		}
		if v == "false" || v == "FALSE" || v == "False" {
			return false
		}

		// convert to unsigned integers if that works
		if ui, err := strconv.ParseUint(v, 10, 64); err == nil {
			return ui
		}

		// convert to signed integers if that works
		if si, err := strconv.ParseInt(v, 10, 64); err == nil {
			return si
		}

		// convert to float if that works
		if fl, err := strconv.ParseFloat(v, 64); err == nil {
			return fl
		}

	}

	// no conversion possible, just return
	return value
}
