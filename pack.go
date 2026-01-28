package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

//go:embed tpl/entry.tpl.go
var entryGoTemplate string

func pack(projectDir, compilerName, luaMain, outputBin string) {
	fmt.Println("ropacker 将会打包", projectDir, "内的所有文件, 并使用", compilerName, "作为解释器, 打包产物为", outputBin, ", 运行时将执行", luaMain, "内的代码")

	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		panic(fmt.Errorf("%s not exist", projectDir))
	}

	fmt.Println("正在解析执行器代码...")

	tmpEntryGo := filepath.Join(os.TempDir(), "lua_pack_entry.go")
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

	zipBuf := new(bytes.Buffer)
	zipW := zip.NewWriter(zipBuf)

	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		relPath, _ := filepath.Rel(projectDir, path)
		zipFile, _ := zipW.Create(relPath)

		fileContent, _ := os.ReadFile(path)
		zipFile.Write(fileContent)

		return nil
	})

	if err != nil {
		panic(fmt.Errorf("can not write zip: %v", err))
	}

	zipW.Close()

	fmt.Println("正在编译执行器...")

	tmpBin := filepath.Join(os.TempDir(), "lua_pack_tmp_bin")
	if runtime.GOOS == "windows" {
		tmpBin += ".exe"
	}

	defer os.Remove(tmpBin)

	buildCmd := exec.Command("go", "build", "-ldflags=-s -w -extldflags=-static -X main.version= -X main.commit=", "-o", tmpBin, tmpEntryGo)
	buildCmd.Env = os.Environ()
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		panic(fmt.Errorf("can not execute go build: %v", err))
	}

	fmt.Println("正在生成最终产物...")

	tmpBinContent, _ := os.ReadFile(tmpBin)

	outFile, err := os.Create(outputBin)
	if err != nil {
		panic(fmt.Errorf("can not create output file: %v", err))
	}

	defer outFile.Close()

	outFile.Write(tmpBinContent)
	outFile.Write([]byte("===LUA_PACK_ZIP_START==="))
	outFile.Write(zipBuf.Bytes())

	fmt.Println("打包成功, 产物文件:", outputBin)
}
