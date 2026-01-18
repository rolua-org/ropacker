package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	tmpDir, err := os.MkdirTemp("", "lua-pack-*")
	if err != nil {
		panic(fmt.Errorf("can not create temp dir: %v", err))
	}

	defer os.RemoveAll(tmpDir)

	exePath, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("can not get self path: %v", err))
	}

	f, err := os.Open(exePath)
	if err != nil {
		panic(fmt.Errorf("can not open self bin: %v", err))
	}

	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		panic(fmt.Errorf("can not read self bin: %v", err))
	}

	zipMarker := []byte("===LUA_PACK_ZIP_START===")
	_, zipData, found := bytes.Cut(content, zipMarker)
	if !found {
		panic(fmt.Errorf("can not find marker"))
	}

	zipTmpFile := filepath.Join(tmpDir, "project.zip")
	if err := os.WriteFile(zipTmpFile, zipData, 0644); err != nil {
		panic(fmt.Errorf("can not write temp zip: %v", err))
	}

	if err := unzip(zipTmpFile, tmpDir); err != nil {
		panic(fmt.Errorf("can not extract zip: %v", err))
	}

	if err := os.Chdir(tmpDir); err != nil {
		panic(fmt.Errorf("can not change work dir: %v", err))
	}

	compilerRelPath := "{{.CompilerPath}}"
	luaMain := "{{.LuaMainPath}}"

	compilerAbsPath := filepath.Join(tmpDir, compilerRelPath)

	if runtime.GOOS == "windows" {
		compilerAbsPath = strings.ReplaceAll(compilerAbsPath, "/", "\\")
		luaMain = strings.ReplaceAll(luaMain, "/", "\\")
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(compilerAbsPath, 0755); err != nil {
			panic(fmt.Errorf("can not change mod: %v | path: %s", err, compilerAbsPath))
		}
	}

	cmdArgs := append([]string{luaMain}, os.Args[1:]...)

	cmd := exec.Command(compilerAbsPath, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("can not execute lua: %v | compiler path: %s", err, compilerAbsPath))
	}
}

func unzip(zipFile, destDir string) error {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}

	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)

		inFile, err := f.Open()
		if err != nil {
			return err
		}

		defer inFile.Close()

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		defer outFile.Close()

		io.Copy(outFile, inFile)
	}

	return nil
}
