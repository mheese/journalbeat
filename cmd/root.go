package cmd

import (
	"github.com/elastic/beats/libbeat/cmd"
	"github.com/mheese/journalbeat/beater"
)

// Name of this beat
var Name = "journalbeat"

// RootCmd to handle beats cli
var RootCmd = cmd.GenRootCmd(Name, "", beater.New)
