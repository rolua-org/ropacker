package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

func packMode(args []string) {
	if len(args) < 5 {
		fmt.Println("Usage: ropacker -pack <project-dir> <compiler-name> <lua-main> [output-bin]")
		return
	}

	projDir := args[2]
	compilerName := args[3]
	luaEntry := args[4]

	if len(args) == 6 {
		OutputBinary = args[5]
	}

	compilerFullPath := filepath.Join(projDir, compilerName)
	luaEntryFullPath := filepath.Join(projDir, luaEntry)

	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		fmt.Println("can not find project-dir:", err)
		return
	}

	if _, err := os.Stat(compilerFullPath); os.IsNotExist(err) {
		fmt.Println("can not find compiler-name:", err)
		return
	}
	if _, err := os.Stat(luaEntryFullPath); os.IsNotExist(err) {
		fmt.Println("can not find lua-main:", err)
		return
	}

	fmt.Println("Building...")

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)

	_ = filepath.Walk(projDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel(projDir, filePath)
		zipFile, _ := zipWriter.Create(relPath)

		fileData, _ := os.ReadFile(filePath)
		zipFile.Write(fileData)

		return nil
	})

	zipWriter.Close()
	zipData := zipBuf.Bytes()

	selfBinary, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Println("can not read ropacker-bin:", err)
		return
	}

	dataBuf := new(bytes.Buffer)
	dataBuf.WriteString(DataMarker)

	binary.Write(dataBuf, binary.LittleEndian, uint32(len(compilerName)))
	dataBuf.WriteString(compilerName)

	binary.Write(dataBuf, binary.LittleEndian, uint32(len(luaEntry)))
	dataBuf.WriteString(luaEntry)

	binary.Write(dataBuf, binary.LittleEndian, uint32(len(zipData)))
	dataBuf.Write(zipData)

	outputFile, err := os.Create(OutputBinary)
	if err != nil {
		fmt.Println("can not create output-bin:", err)
		return
	}

	defer outputFile.Close()

	outputFile.Write(selfBinary)
	outputFile.Write(dataBuf.Bytes())

	os.Chmod(OutputBinary, 0777)

	fmt.Println("Build success! Output to", OutputBinary)
}
