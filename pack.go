package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const DataMarker = "###ROPACKER_DATA_PART_START###"

//go:embed embed/*
var runFS embed.FS

func pack(project_dir, compiler_name, lua_main, output_bin string) error {
	compiler_name_path := filepath.Join(project_dir, compiler_name)
	lua_main_path := filepath.Join(project_dir, lua_main)

	if _, err := os.Stat(project_dir); os.IsNotExist(err) {
		return fmt.Errorf("project-dir not exist: %v", err)
	}

	if _, err := os.Stat(compiler_name_path); os.IsNotExist(err) {
		return fmt.Errorf("compiler-name not exist: %v", err)
	}

	if _, err := os.Stat(lua_main_path); os.IsNotExist(err) {
		return fmt.Errorf("lua-main not exist: %v", err)
	}

	fmt.Println("Building...")

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)

	defer zipWriter.Close()

	err := filepath.Walk(project_dir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel(project_dir, filePath)
		zipFile, _ := zipWriter.Create(relPath)

		fileData, _ := os.ReadFile(filePath)
		zipFile.Write(fileData)

		return nil
	})

	if err != nil {
		return fmt.Errorf("compress zip failed: %v", err)
	}

	zipData := zipBuf.Bytes()

	tempDir, err := os.MkdirTemp("", "rolua_boot_")
	if err != nil {
		return fmt.Errorf("create temp dir failed: %v", err)
	}

	defer os.RemoveAll(tempDir)

	bootSourcePath := filepath.Join(tempDir, "run.go")
	bootBinaryPath := filepath.Join(tempDir, "run")

	templateData, err := runFS.ReadFile("embed/run.go")
	if err != nil {
		return fmt.Errorf("read run.go failed: %v", err)
	}

	os.WriteFile(bootSourcePath, templateData, 0644)

	cmd := exec.Command("go", "build", "-o", bootBinaryPath, bootSourcePath)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile bootstrap failed: %v", err)
	}

	bootBinary, err := os.ReadFile(bootBinaryPath)
	if err != nil {
		return fmt.Errorf("read bootstrap binary failed: %v", err)
	}

	dataBuf := new(bytes.Buffer)
	dataBuf.WriteString(DataMarker)

	binary.Write(dataBuf, binary.LittleEndian, uint32(len(compiler_name)))
	dataBuf.WriteString(compiler_name)

	binary.Write(dataBuf, binary.LittleEndian, uint32(len(lua_main)))
	dataBuf.WriteString(lua_main)

	binary.Write(dataBuf, binary.LittleEndian, uint32(len(zipData)))
	dataBuf.Write(zipData)

	outputFile, err := os.Create(output_bin)
	if err != nil {
		return fmt.Errorf("create output-bin failed: %v", err)
	}

	defer outputFile.Close()

	outputFile.Write(bootBinary)
	outputFile.Write(dataBuf.Bytes())

	os.Chmod(output_bin, 0777)

	fmt.Println("Build success! Output to", output_bin)
	return nil
}
