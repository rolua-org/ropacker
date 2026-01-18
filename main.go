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

	project_dir := args[1]
	compiler_name := args[2]
	lua_main := args[3]
	output_bin := "rolua-packed"

	if len(args) == 5 {
		output_bin = args[4]

	} else {
		if runtime.GOOS == "windows" {
			output_bin = output_bin + ".exe"
		}
	}

	if err := pack(project_dir, compiler_name, lua_main, output_bin); err != nil {
		fmt.Println("pack failed:", err)
	}
}
