package main

import (
	"os"
)

const (
	PackFlag      = "-pack"
	DataMarker    = "###ROLUA_DATA###"
	TempDirPrefix = "rolua_pack_"
)

var OutputBinary = "rolua-app"

func main() {
	args := os.Args

	if len(args) >= 2 && args[1] == PackFlag {
		packMode(args)
		return
	}

	runMode()
}
