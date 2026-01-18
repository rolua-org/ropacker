package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	DataMarker    = "###ROPACKER_DATA_PART_START###"
	TempDirPrefix = "ropacker_run_"
)

func readBytes(r *bytes.Reader, n int) []byte {
	b := make([]byte, n)
	r.Read(b)

	return b
}

func main() {
	selfPath := os.Args[0]

	selfBinary, err := os.ReadFile(selfPath)
	if err != nil {
		fmt.Println("can not read self-bin:", err)
		return
	}

	markerPos := bytes.LastIndex(selfBinary, []byte(DataMarker))
	if markerPos == -1 {
		fmt.Println(selfPath, "is not a packed binary")
		return
	}

	dataBuf := selfBinary[markerPos+len(DataMarker):]
	dataReader := bytes.NewReader(dataBuf)

	var compilerNameLen, luaEntryLen, zipDataLen uint32

	binary.Read(dataReader, binary.LittleEndian, &compilerNameLen)
	compilerName := string(readBytes(dataReader, int(compilerNameLen)))

	binary.Read(dataReader, binary.LittleEndian, &luaEntryLen)
	luaEntry := string(readBytes(dataReader, int(luaEntryLen)))

	binary.Read(dataReader, binary.LittleEndian, &zipDataLen)
	zipData := readBytes(dataReader, int(zipDataLen))

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		fmt.Println("can not create dir:", err)
		return
	}

	defer os.RemoveAll(tempDir)

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		fmt.Println("can not extract zip:", err)
		return
	}

	for _, zipFile := range zipReader.File {
		targetPath := filepath.Join(tempDir, zipFile.Name)
		os.MkdirAll(filepath.Dir(targetPath), 0777)

		srcFile, _ := zipFile.Open()
		dstFile, _ := os.Create(targetPath)
		io.Copy(dstFile, srcFile)

		srcFile.Close()
		dstFile.Close()
	}

	compilerPath := filepath.Join(tempDir, compilerName)
	os.Chmod(compilerPath, 0777)

	os.Chdir(tempDir)

	cmd := exec.Command(compilerPath, luaEntry)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()
}
