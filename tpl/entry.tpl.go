package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/klauspost/compress/zstd"
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

	zstdMarker := []byte("===LUA_PACK_START===")
	_, zstdData, found := bytes.Cut(content, zstdMarker)
	if !found {
		panic(fmt.Errorf("can not find zstd marker"))
	}

	if err := untarZstd(zstdData, tmpDir); err != nil {
		panic(fmt.Errorf("can not extract zstd/tar: %v", err))
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

func untarZstd(data []byte, destDir string) error {
	zr, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}

	defer zr.Close()

	tr := tar.NewReader(zr)

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {

		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}

			f.Close()

		}
	}

	return nil
}
