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
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/elastic/beats/libbeat/common"
)

// SyslogFacilityString is a map containing the textual equivalence of a given facility number
var SyslogFacilityString = map[string]string{
	"0":  "kernel",
	"1":  "user",
	"2":  "mail",
	"3":  "daemon",
	"4":  "auth",
	"5":  "syslog",
	"6":  "line printer",
	"7":  "network news",
	"8":  "uucp",
	"9":  "clock daemon",
	"10": "security/auth",
	"11": "ftp",
	"12": "ntp",
	"13": "log audit",
	"14": "log alert",
	"15": "clock daemon",
	"16": "local0",
	"17": "local1",
	"18": "local2",
	"19": "local3",
	"20": "local4",
	"21": "local5",
	"22": "local6",
	"23": "local7",
}

// PriorityConversionMap is a map containing the textual equivalence of a given priority string number
var PriorityConversionMap = map[string]string{
	"0": "emergency",
	"1": "alert",
	"2": "critical",
	"3": "error",
	"4": "warning",
	"5": "notice",
	"6": "informational",
	"7": "debug",
}

// MapStrFromJournalEntry takes a JournalD entry and converts it to an event
// that is more compatible with the Elasitc products. It will perform the
// following additional steps to an event:
// - lowercase all fields (seriously, who wants to type caps all day?!?)
// - remove underscores from the beginning of fields as they are reserved in
//   ElasticSearch for metadata information
// - fields that can be converted to numbers, will be converted to numbers
func MapStrFromJournalEntry(ev *sdjournal.JournalEntry, cleanKeys bool, convertToNumbers bool, MoveMetadataLocation string, ParsePriority bool, ParseFacility bool) common.MapStr {
	m := common.MapStr{}
	// for the sake of MoveMetadataLocation we will write all the JournalEntry data except the "message" here
	target := m

	// convert non-empty MoveMetadataLocation to the nested common.MapStr{} and point target to the deepest one
	if MoveMetadataLocation != "" {
		dests := strings.Split(MoveMetadataLocation, ".")
		for _, key := range dests {
			target[key] = common.MapStr{}
			target = target[key].(common.MapStr)
		}
	}

	// range over the JournalEntry Fields and convert to the common.MapStr
	for k, v := range ev.Fields {
		nk := makeNewKey(k, cleanKeys)
		if nk == "priority" && ParsePriority {
			v = PriorityConversionMap[v]
		}
		if nk == "syslog_facility" && ParseFacility {
			v = PriorityConversionMap[v]
		}
		nv := makeNewValue(v, convertToNumbers)
		// message Field should be on the top level of the event
		if nk == "message" {
			m[nk] = nv
			continue
		}
		target[nk] = nv
	}

	return m
}

func makeNewKey(key string, cleanKeys bool) string {
	if !cleanKeys {
		return key
	}

	return strings.TrimLeft(strings.ToLower(key), "_")
}

func makeNewValue(value string, convertToNumbers bool) interface{} {
	switch value {
	// convert booleans if possible
	// strconv.ParseBool is unfortunately too forgiving,
	// we only want the hard words
	case "true", "TRUE", "True":
		return true
	case "false", "FALSE", "False":
		return false
	default:
		if !convertToNumbers {
			return value
		}
		// convert to unsigned integers if that works
		if ui, err := strconv.ParseUint(value, 10, 64); err == nil {
			return ui
		}
		// convert to signed integers if that works
		if si, err := strconv.ParseInt(value, 10, 64); err == nil {
			return si
		}
		// convert to float if that works
		if fl, err := strconv.ParseFloat(value, 64); err == nil {
			return fl
		}
		return value
	}
}
