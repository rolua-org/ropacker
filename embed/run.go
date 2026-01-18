package main

import (
	"archive/zip"
	"bytes"
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

func readBytes(r *bytes.Reader, n int) ([]byte, error) {
	b := make([]byte, n)

	readN, err := r.Read(b)
	if err != nil {
		return nil, fmt.Errorf("can not read bytes: %v", err)
	}

	if readN != n {
		return nil, fmt.Errorf("only read %d, need %d", readN, n)
	}

	return b, nil
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

	compilerNameBytes, err := readBytes(dataReader, int(compilerNameLen))
	if err != nil {
		fmt.Println("can not read compiler-name bytes:", err)
		return
	}

	compilerName := string(compilerNameBytes)

	luaEntryBytes, err := readBytes(dataReader, int(luaEntryLen))
	if err != nil {
		fmt.Println("can not read lua-main bytes:", err)
		return
	}

	luaEntry := string(luaEntryBytes)

	zipDataBytes, err := readBytes(dataReader, int(zipDataLen))
	if err != nil {
		fmt.Println("can not read zip bytes:", err)
		return
	}

	zipData := zipDataBytes

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
