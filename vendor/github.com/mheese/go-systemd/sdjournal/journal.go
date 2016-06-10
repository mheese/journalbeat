// Copyright 2015 RedHat, Inc.
// Copyright 2015 CoreOS, Inc.
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

// Package sdjournal provides a low-level Go interface to the
// systemd journal wrapped around the sd-journal C API.
//
// All public read methods map closely to the sd-journal API functions. See the
// sd-journal.h documentation[1] for information about each function.
//
// To write to the journal, see the pure-Go "journal" package
//
// [1] http://www.freedesktop.org/software/systemd/man/sd-journal.html
package sdjournal

/*
#cgo pkg-config: libsystemd
#include <systemd/sd-journal.h>
#include <systemd/sd-id128.h>
#include <stdlib.h>
#include <syslog.h>
*/
import "C"
import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"unsafe"
)

// Journal entry field strings which correspond to:
// http://www.freedesktop.org/software/systemd/man/systemd.journal-fields.html
const (
	SD_JOURNAL_FIELD_SYSTEMD_UNIT = "_SYSTEMD_UNIT"
	SD_JOURNAL_FIELD_MESSAGE      = "MESSAGE"
	SD_JOURNAL_FIELD_PID          = "_PID"
	SD_JOURNAL_FIELD_UID          = "_UID"
	SD_JOURNAL_FIELD_GID          = "_GID"
	SD_JOURNAL_FIELD_HOSTNAME     = "_HOSTNAME"
	SD_JOURNAL_FIELD_MACHINE_ID   = "_MACHINE_ID"
)

// Journal event constants
const (
	SD_JOURNAL_NOP        = int(C.SD_JOURNAL_NOP)
	SD_JOURNAL_APPEND     = int(C.SD_JOURNAL_APPEND)
	SD_JOURNAL_INVALIDATE = int(C.SD_JOURNAL_INVALIDATE)
)

const (
	// IndefiniteWait is a sentinel value that can be passed to
	// sdjournal.Wait() to signal an indefinite wait for new journal
	// events. It is implemented as the maximum value for a time.Duration:
	// https://github.com/golang/go/blob/e4dcf5c8c22d98ac9eac7b9b226596229624cb1d/src/time/time.go#L434
	IndefiniteWait time.Duration = 1<<63 - 1
)

// Journal is a Go wrapper of an sd_journal structure.
type Journal struct {
	cjournal *C.sd_journal
	mu       sync.Mutex
}

// JournalEntry is an alias for map[string]interface{}
type JournalEntry map[string]interface{}

// Match is a convenience wrapper to describe filters supplied to AddMatch.
type Match struct {
	Field string
	Value string
}

// String returns a string representation of a Match suitable for use with AddMatch.
func (m *Match) String() string {
	return m.Field + "=" + m.Value
}

// NewJournal returns a new Journal instance pointing to the local journal
func NewJournal() (*Journal, error) {
	j := &Journal{}
	r := C.sd_journal_open(&j.cjournal, C.SD_JOURNAL_LOCAL_ONLY)

	if r < 0 {
		return nil, fmt.Errorf("failed to open journal: %d", r)
	}

	return j, nil
}

// NewJournalFromDir returns a new Journal instance pointing to a journal residing
// in a given directory. The supplied path may be relative or absolute; if
// relative, it will be converted to an absolute path before being opened.
func NewJournalFromDir(path string) (*Journal, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))

	j := &Journal{}
	r := C.sd_journal_open_directory(&j.cjournal, p, 0)
	if r < 0 {
		return nil, fmt.Errorf("failed to open journal in directory %q: %d", path, r)
	}

	return j, nil
}

// Close closes a journal opened with NewJournal.
func (j *Journal) Close() error {
	j.mu.Lock()
	C.sd_journal_close(j.cjournal)
	j.mu.Unlock()

	return nil
}

// AddMatch adds a match by which to filter the entries of the journal.
func (j *Journal) AddMatch(match string) error {
	m := C.CString(match)
	defer C.free(unsafe.Pointer(m))

	j.mu.Lock()
	r := C.sd_journal_add_match(j.cjournal, unsafe.Pointer(m), C.size_t(len(match)))
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to add match: %d", r)
	}

	return nil
}

// AddDisjunction inserts a logical OR in the match list.
func (j *Journal) AddDisjunction() error {
	j.mu.Lock()
	r := C.sd_journal_add_disjunction(j.cjournal)
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to add a disjunction in the match list: %d", r)
	}

	return nil
}

// AddConjunction inserts a logical AND in the match list.
func (j *Journal) AddConjunction() error {
	j.mu.Lock()
	r := C.sd_journal_add_conjunction(j.cjournal)
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to add a conjunction in the match list: %d", r)
	}

	return nil
}

// FlushMatches flushes all matches, disjunctions and conjunctions.
func (j *Journal) FlushMatches() {
	j.mu.Lock()
	C.sd_journal_flush_matches(j.cjournal)
	j.mu.Unlock()
}

// Next advances the read pointer into the journal by one entry.
func (j *Journal) Next() (int, error) {
	j.mu.Lock()
	r := C.sd_journal_next(j.cjournal)
	j.mu.Unlock()

	if r < 0 {
		return int(r), fmt.Errorf("failed to iterate journal: %d", r)
	}

	return int(r), nil
}

// NextSkip advances the read pointer by multiple entries at once,
// as specified by the skip parameter.
func (j *Journal) NextSkip(skip uint64) (uint64, error) {
	j.mu.Lock()
	r := C.sd_journal_next_skip(j.cjournal, C.uint64_t(skip))
	j.mu.Unlock()

	if r < 0 {
		return uint64(r), fmt.Errorf("failed to iterate journal: %d", r)
	}

	return uint64(r), nil
}

// Previous sets the read pointer into the journal back by one entry.
func (j *Journal) Previous() (uint64, error) {
	j.mu.Lock()
	r := C.sd_journal_previous(j.cjournal)
	j.mu.Unlock()

	if r < 0 {
		return uint64(r), fmt.Errorf("failed to iterate journal: %d", r)
	}

	return uint64(r), nil
}

// PreviousSkip sets back the read pointer by multiple entries at once,
// as specified by the skip parameter.
func (j *Journal) PreviousSkip(skip uint64) (uint64, error) {
	j.mu.Lock()
	r := C.sd_journal_previous_skip(j.cjournal, C.uint64_t(skip))
	j.mu.Unlock()

	if r < 0 {
		return uint64(r), fmt.Errorf("failed to iterate journal: %d", r)
	}

	return uint64(r), nil
}

// GetData gets the data object associated with a specific field from the
// current journal entry.
func (j *Journal) GetData(field string) (string, error) {
	f := C.CString(field)
	defer C.free(unsafe.Pointer(f))

	var d unsafe.Pointer
	var l C.size_t

	j.mu.Lock()
	r := C.sd_journal_get_data(j.cjournal, f, &d, &l)
	j.mu.Unlock()

	if r < 0 {
		return "", fmt.Errorf("failed to read message: %d", r)
	}

	msg := C.GoStringN((*C.char)(d), C.int(l))

	return msg, nil
}
func splitNameValue(fieldData []byte) (string, []byte) {
	var field string
	var value []byte
	for i, r := range fieldData {
		if r == '=' {
			field = string(fieldData[:i])
			value = fieldData[i+1:]
			return field, value
		}
	}
	return "", []byte{}
}
func addToMap(hashmap JournalEntry, name string, value []byte) {
	v, ok := hashmap[name]
	if !ok {
		// if the field does not exist, simply add the value
		if utf8.Valid(value) {
			hashmap[name] = string(value)
		} else {
			hashmap[name] = value
		}
	} else {
		// if the field does exist, make it a slice and append
		switch t := v.(type) {
		default:
			fmt.Printf("Unexpected type: %T\n", t)
		case string:
			// NOTE: it is assumed here that consecutive fields with the same name are also UTF-8 strings
			hashmap[name] = []string{t, string(value)}
		case []byte:
			hashmap[name] = [][]byte{t, value}
		case []string:
			// NOTE: it is assumed here that consecutive fields with the same name are also UTF-8 strings
			hashmap[name] = append(t, string(value))
		case [][]byte:
			hashmap[name] = append(t, value)
		}
	}
}

func (j *Journal) GetDataAll() (JournalEntry, error) {
	data := make(JournalEntry)

	var d unsafe.Pointer
	var l C.size_t
	var cboot_id C.sd_id128_t
	var csid = C.CString("123456789012345678901234567890123")
	defer C.free(unsafe.Pointer(csid))
	var crealtime C.uint64_t
	var cmonotonic C.uint64_t
	var ccursor *C.char
	j.mu.Lock()
	// not in their own fields
	C.sd_journal_set_data_threshold(j.cjournal, 0)
	C.sd_journal_get_realtime_usec(j.cjournal, &crealtime)
	C.sd_journal_get_monotonic_usec(j.cjournal, &cmonotonic, &cboot_id)
	C.sd_id128_to_string(cboot_id, csid)
	C.sd_journal_get_cursor(j.cjournal, (**C.char)(&ccursor))
	defer C.free(unsafe.Pointer(ccursor))

	// reset to start the loop
	C.sd_journal_restart_data(j.cjournal)
	j.mu.Unlock()

	realtime := uint64(crealtime)
	monotonic := uint64(cmonotonic)
	cursor := C.GoString(ccursor)
	bootid := C.GoString(csid)

	data["__CURSOR"] = cursor
	data["__REALTIME_TIMESTAMP"] = realtime
	data["__MONOTONIC_TIMESTAMP"] = monotonic
	data["__BOOT_ID"] = bootid

	for {
		// retrieve new field
		j.mu.Lock()
		r := C.sd_journal_enumerate_data(j.cjournal, &d, &l)
		j.mu.Unlock()

		if r <= 0 {

			break
		}

		fieldData := C.GoBytes(d, C.int(l))
		name, value := splitNameValue(fieldData)
		addToMap(data, name, value)
	}

	// Add catalog data as well if there is a MESSAGE_ID
	_, ok := data["MESSAGE_ID"]
	if ok {
		catalogEntry, err := j.GetCatalog()
		if err == nil {
			data["CATALOG_ENTRY"] = catalogEntry
		}
	}

	return data, nil
}

// GetDataValue gets the data object associated with a specific field from the
// current journal entry, returning only the value of the object.
func (j *Journal) GetDataValue(field string) (string, error) {
	val, err := j.GetData(field)
	if err != nil {
		return "", err
	}
	return strings.SplitN(val, "=", 2)[1], nil
}

// SetDataThresold sets the data field size threshold for data returned by
// GetData. To retrieve the complete data fields this threshold should be
// turned off by setting it to 0, so that the library always returns the
// complete data objects.
func (j *Journal) SetDataThreshold(threshold uint64) error {
	j.mu.Lock()
	r := C.sd_journal_set_data_threshold(j.cjournal, C.size_t(threshold))
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to set data threshold: %d", r)
	}

	return nil
}

// GetRealtimeUsec gets the realtime (wallclock) timestamp of the current
// journal entry.
func (j *Journal) GetRealtimeUsec() (uint64, error) {
	var usec C.uint64_t

	j.mu.Lock()
	r := C.sd_journal_get_realtime_usec(j.cjournal, &usec)
	j.mu.Unlock()

	if r < 0 {
		return 0, fmt.Errorf("error getting timestamp for entry: %d", r)
	}

	return uint64(usec), nil
}

//SeekHead seeks to the beginning of the journal, i.e. the oldest available entry.
func (j *Journal) SeekHead() error {
	j.mu.Lock()
	r := C.sd_journal_seek_head(j.cjournal)
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to seek to head of journal: %d", r)
	}

	return nil
}

// SeekTail may be used to seek to the end of the journal, i.e. the most recent
// available entry.
func (j *Journal) SeekTail() error {
	j.mu.Lock()
	r := C.sd_journal_seek_tail(j.cjournal)
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to seek to tail of journal: %d", r)
	}

	return nil
}

// SeekMonotonicUsec seeks to the entry with the specified monotonic timestamp,
// i.e. CLOCK_MONOTONIC. Since monotonic time restarts on every reboot a boot ID needs
// to be specified as well.
func (j *Journal) SeekMonotonicUsec(boot_id string, usec uint64) error {
	// get the boot_id first
	cs := C.CString(boot_id)
	defer C.free(unsafe.Pointer(cs))
	var cboot_id C.sd_id128_t
	r := C.sd_id128_from_string(cs, &cboot_id)
	if r < 0 {
		return fmt.Errorf("failed to retrieve 128bit ID from string '%s': %d", boot_id, r)
	}

	j.mu.Lock()
	r = C.sd_journal_seek_monotonic_usec(j.cjournal, cboot_id, C.uint64_t(usec))
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to seek to monotonic_clock(%s, %d): %d", boot_id, usec, r)
	}
	return nil
}

// SeekRealtimeUsec seeks to the entry with the specified realtime (wallclock)
// timestamp, i.e. CLOCK_REALTIME.
func (j *Journal) SeekRealtimeUsec(usec uint64) error {
	j.mu.Lock()
	r := C.sd_journal_seek_realtime_usec(j.cjournal, C.uint64_t(usec))
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to seek to realtime_clock(%d): %d", usec, r)
	}

	return nil
}

// SeekCursor seeks to the entry located at the specified cursor string. If no entry
// matching the specified cursor is found the call will seek to the next closest entry
// (in terms of time) instead. SeekCursor returns true if it was able to seek to the
// exact postion, or false, if it was able to only seek to the next closest position.
// It returns an error if the operation failed completely
func (j *Journal) SeekCursor(cursor string) error {
	ccursor := C.CString(cursor)
	defer C.free(unsafe.Pointer(ccursor))

	j.mu.Lock()
	r := C.sd_journal_seek_cursor(j.cjournal, ccursor)
	j.mu.Unlock()

	if r < 0 {
		return fmt.Errorf("failed to seek to cursor '%s': %d", cursor, r)
	}

	return nil
}

// GetCursor returns a cursor string for the current journal entry. A cursor is a serialization of the current journal position formatted as text. The string only contains printable characters and can be passed around in text form. The cursor identifies a journal entry globally and in a stable way and may be used to later seek to it via SeekCursor.
func (j *Journal) GetCursor() (string, error) {
	var ccursor *C.char

	j.mu.Lock()
	r := C.sd_journal_get_cursor(j.cjournal, (**C.char)(&ccursor))
	j.mu.Unlock()

	defer C.free(unsafe.Pointer(ccursor))

	if r < 0 {
		return "", fmt.Errorf("failed to get cursor: %d", r)
	}

	return C.GoString(ccursor), nil
}

// TestCursor  may be used to check whether the current position in the journal matches the specified cursor. This is useful since cursor strings do not uniquely identify an entry: the same entry might be referred to by multiple different cursor strings, and hence string comparing cursors is not possible. Use this call to verify after an invocation of SeekCursor whether the entry being sought to was actually found in the journal or the next closest entry was used instead.
func (j *Journal) TestCursor(cursor string) (bool, error) {
	ccursor := C.CString(cursor)
	defer C.free(unsafe.Pointer(ccursor))

	j.mu.Lock()
	r := C.sd_journal_test_cursor(j.cjournal, ccursor)
	j.mu.Unlock()

	// testing failed if negative zero
	if r < 0 {
		return false, fmt.Errorf("failed to test cursor '%s' for seek accuracy: %d", cursor, r)
	}

	// if 0, it sought to the next closest position
	if r == 0 {
		return false, nil
	}

	// if positive, it sought to the exact position
	return true, nil
}

// GetCatalog retrieves a message catalog entry for the current journal entry.
// This will look up an entry in the message catalog by using the "MESSAGE_ID="
// field of the current journal entry. Before returning the entry all journal
// field names in the catalog entry text enclosed in "@" will be replaced by the
// respective field values of the current entry. If a field name referenced in
// the message catalog entry does not exist, in the current journal entry, the
// "@" will be removed, but the field name otherwise left untouched.
func (j *Journal) GetCatalog() (string, error) {
	var ccatalog *C.char

	j.mu.Lock()
	r := C.sd_journal_get_catalog(j.cjournal, (**C.char)(&ccatalog))
	j.mu.Unlock()

	defer C.free(unsafe.Pointer(ccatalog))

	if r < 0 {
		return "", fmt.Errorf("failed to retrieve catalog entry for current journal entry: %d", r)
	}

	catalog := C.GoString(ccatalog)
	return catalog, nil
}

// GetCatalogForMessageID works similar to GetCatalog(), but the entry is looked
// up by the specified message ID (no open journal context is necessary for
// this), and no field substitution is performed.
func GetCatalogForMessageID(messageId string) (string, error) {
	cmessageId := C.CString(messageId)
	defer C.free(unsafe.Pointer(cmessageId))

	var mid C.sd_id128_t
	r := C.sd_id128_from_string(cmessageId, &mid)
	if r < 0 {
		return "", fmt.Errorf("failed to get sd_id128_t from provided MESSAGE_ID '%s': %d", messageId, r)
	}

	var ccatalog *C.char
	r = C.sd_journal_get_catalog_for_message_id(mid, (**C.char)(&ccatalog))
	defer C.free(unsafe.Pointer(ccatalog))

	if r < 0 {
		return "", fmt.Errorf("failed to retrieve catalog entry for MESSAGE_ID '%s': %d", messageId, r)
	}

	catalog := C.GoString(ccatalog)
	return catalog, nil
}

// Wait will synchronously wait until the journal gets changed. The maximum time
// this call sleeps may be controlled with the timeout parameter.  If
// sdjournal.IndefiniteWait is passed as the timeout parameter, Wait will
// wait indefinitely for a journal change.
func (j *Journal) Wait(timeout time.Duration) int {
	var to uint64
	if timeout == IndefiniteWait {
		// sd_journal_wait(3) calls for a (uint64_t) -1 to be passed to signify
		// indefinite wait, but using a -1 overflows our C.uint64_t, so we use an
		// equivalent hex value.
		to = 0xffffffffffffffff
	} else {
		to = uint64(time.Now().Add(timeout).Unix() / 1000)
	}
	j.mu.Lock()
	r := C.sd_journal_wait(j.cjournal, C.uint64_t(to))
	j.mu.Unlock()

	return int(r)
}

// GetUsage returns the journal disk space usage, in bytes.
func (j *Journal) GetUsage() (uint64, error) {
	var out C.uint64_t
	j.mu.Lock()
	r := C.sd_journal_get_usage(j.cjournal, &out)
	j.mu.Unlock()

	if r < 0 {
		return 0, fmt.Errorf("failed to get journal disk space usage: %d", r)
	}

	return uint64(out), nil
}
