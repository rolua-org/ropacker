package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	args := os.Args

	if len(args) < 4 {
		fmt.Println("Usage: ropacker <project-dir> <compiler-name> <lua-main> [output-bin]")
		return
	}

	projectDir := args[1]
	compilerName := args[2]
	luaMain := args[3]
	outputBin := "rolua-packed"

	if len(args) == 5 {
		outputBin = args[4]

	} else {
		if runtime.GOOS == "windows" {
			outputBin = outputBin + ".exe"
		}
	}

	pack(projectDir, compilerName, luaMain, outputBin)
}
