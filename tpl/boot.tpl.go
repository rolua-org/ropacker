package main

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	compiler string = "{{.compiler}}"
	lua      string = "{{.lua}}"
)

func main() {
	self, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("can not get self: %v", err))
	}

	selfbin, err := os.Open(self)
	if err != nil {
		panic(fmt.Errorf("can not open self bin: %v", err))
	}

	defer selfbin.Close()

	selfi, err := selfbin.Stat()
	if err != nil {
		panic(fmt.Errorf("can not get self bin info: %v", err))
	}

	tmp, err := os.MkdirTemp(os.TempDir(), "boot-job-*")
	if err != nil {
		panic(fmt.Errorf("can not create temp dir: %v", err))
	}

	defer os.RemoveAll(tmp)

	selfbin.Seek(-8, io.SeekEnd)

	var bootsize uint64
	binary.Read(selfbin, binary.LittleEndian, &bootsize)
	zipsize := selfi.Size() - int64(bootsize) - 8

	sr := io.NewSectionReader(selfbin, int64(bootsize), zipsize)
	uncompress(sr, zipsize, tmp)

	chdir(tmp)

	args := append([]string{lua}, os.Args[1:]...)
	run(compiler, args...)
}

func chdir(path string) {
	if err := os.Chdir(path); err != nil {
		panic(fmt.Errorf("can not change work dir: %v", err))
	}
}

func run(name string, arg ...string) {
	cmd := exec.Command(name, arg...)

	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("can not run command: %v", err))
	}
}

func uncompress(src io.ReaderAt, size int64, dst string) {
	r, err := zip.NewReader(src, size)
	if err != nil {
		panic(fmt.Errorf("can not read zip: %v", err))
	}

	for _, f := range r.File {
		fp := filepath.Join(dst, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fp, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			panic(fmt.Errorf("can not create file base dir: %v", err))
		}

		fdst, err := os.Create(fp)
		if err != nil {
			panic(fmt.Errorf("can not create file: %v", err))
		}

		fsrc, err := f.Open()
		if err != nil {
			fdst.Close()
			panic(fmt.Errorf("can not read source file: %v", err))
		}

		_, copyerr := io.Copy(fdst, fsrc)
		fsrc.Close()
		fdst.Sync()
		fdst.Close()

		if copyerr != nil {
			panic(fmt.Errorf("can not copy file: %v", err))
		}
	}
}
