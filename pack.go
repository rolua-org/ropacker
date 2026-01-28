package main

import (
	"archive/tar"
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/klauspost/compress/zstd"
)

//go:embed tpl/entry.tpl.go
var entryGoTemplate string

func pack(projectDir, compilerName, luaMain, outputBin string) {
	fmt.Println("ropacker 将会打包", projectDir, "内的所有文件, 并使用", compilerName, "作为解释器, 打包产物为", outputBin, ", 运行时将执行", luaMain, "内的代码")

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		panic(fmt.Errorf("%s not exist", projectDir))
	}

	fmt.Println("正在解析执行器代码...")

	buildDir, err := os.MkdirTemp("", "ropacker-build-*")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(buildDir)

	tmpEntryGo := filepath.Join(buildDir, "main.go")
	defer os.Remove(tmpEntryGo)

	tpl, err := template.New("luaEntry").Parse(entryGoTemplate)
	if err != nil {
		panic(fmt.Errorf("can not parse template: %v", err))
	}

	f, err := os.Create(tmpEntryGo)
	if err != nil {
		panic(fmt.Errorf("can not create temp entry file: %v", err))
	}

	tpl.Execute(f, map[string]string{
		"CompilerPath": compilerName,
		"LuaMainPath":  luaMain,
	})

	f.Close()

	fmt.Println("正在打包项目文件...")

	compressedBuf := new(bytes.Buffer)

	zstdW, err := zstd.NewWriter(compressedBuf, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(20)))
	if err != nil {
		panic(fmt.Errorf("failed to initialize zstd writer: %v", err))
	}

	tarW := tar.NewWriter(zstdW)

	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel(projectDir, path)

		header, _ := tar.FileInfoHeader(info, "")
		header.Name = relPath

		if err := tarW.WriteHeader(header); err != nil {
			return err
		}

		fileContent, _ := os.ReadFile(path)
		tarW.Write(fileContent)

		return nil
	})

	if err != nil {
		panic(fmt.Errorf("compression failed: %v", err))
	}

	tarW.Close()
	zstdW.Close()

	fmt.Println("正在编译执行器...")

	tmpBin := filepath.Join(buildDir, "lua_pack_tmp_bin")
	if runtime.GOOS == "windows" {
		tmpBin += ".exe"
	}

	defer os.Remove(tmpBin)

	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("can not get current path: %v", err))
	}

	if err := os.Chdir(buildDir); err != nil {
		panic(fmt.Errorf("can not change work dir: %v", err))
	}

	if err := runCommand("go", "mod", "init", "temp_entry"); err != nil {
		panic(fmt.Errorf("can not execute go mod init: %v", err))
	}

	if err := runCommand("go", "mod", "tidy"); err != nil {
		panic(fmt.Errorf("can not execute go get: %v", err))
	}

	if err := runCommand("go", "build", "-ldflags=-s -w -extldflags=-static -X main.version= -X main.commit=", "-o", tmpBin); err != nil {
		panic(fmt.Errorf("can not execute go build: %v", err))
	}

	if err := os.Chdir(cwd); err != nil {
		panic(fmt.Errorf("can not change work dir: %v", err))
	}

	fmt.Println("正在生成最终产物...")

	tmpBinContent, _ := os.ReadFile(tmpBin)

	outFile, err := os.Create(outputBin)
	if err != nil {
		panic(fmt.Errorf("can not create output file: %v", err))
	}

	defer outFile.Close()

	outFile.Write(tmpBinContent)
	outFile.Write([]byte("===LUA_PACK_START==="))
	outFile.Write(compressedBuf.Bytes())

	fmt.Println("打包成功, 产物文件:", outputBin)
}

func runCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)

	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
