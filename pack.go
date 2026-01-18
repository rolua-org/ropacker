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
	// 1. 补全默认输出路径
	if outputBin == "" {
		outputBin = "lua_app"
		if filepath.Base(os.Args[0]) == "lua_app.exe" || runtime.GOOS == "windows" {
			outputBin += ".exe"
		}
	}

	// 2. 校验项目目录是否存在
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		fmt.Printf("❌ 错误：项目目录 [%s] 不存在\n", projectDir)
		os.Exit(1)
	}

	// 3. 生成临时的入口Go文件（打包完成后自动删除）
	tmpEntryGo := filepath.Join(os.TempDir(), "lua_pack_entry.go")
	defer os.Remove(tmpEntryGo)

	// 4. 渲染模板：注入编译器路径、lua入口路径
	tpl, err := template.New("luaEntry").Parse(entryGoTemplate)
	if err != nil {
		fmt.Printf("❌ 解析入口模板失败: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Create(tmpEntryGo)
	if err != nil {
		fmt.Printf("❌ 创建临时入口文件失败: %v\n", err)
		os.Exit(1)
	}
	_ = tpl.Execute(f, map[string]string{
		"CompilerPath": compilerName,
		"LuaMainPath":  luaMain,
	})
	_ = f.Close()

	// 5. 打包projectDir下所有文件为zip内存缓冲区
	zipBuf := new(bytes.Buffer)
	zipW := zip.NewWriter(zipBuf)
	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(projectDir, path)
		zipFile, _ := zipW.Create(relPath)
		fileContent, _ := os.ReadFile(path)
		_, _ = zipFile.Write(fileContent)
		return nil
	})
	if err != nil {
		fmt.Printf("❌ 打包项目文件失败: %v\n", err)
		os.Exit(1)
	}
	_ = zipW.Close()

	// 6. 调用go build编译临时入口文件，继承所有环境变量
	tmpBin := filepath.Join(os.TempDir(), "lua_pack_tmp_bin")
	if runtime.GOOS == "windows" {
		tmpBin += ".exe"
	}
	defer os.Remove(tmpBin)

	buildCmd := exec.Command("go", "build", "-o", tmpBin, tmpEntryGo)
	buildCmd.Env = os.Environ() // 核心：继承所有系统环境变量
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("❌ Go编译入口文件失败: %v\n", err)
		os.Exit(1)
	}

	// 7. 合并：编译后的二进制 + 标记 + zip数据 = 最终产物
	tmpBinContent, _ := os.ReadFile(tmpBin)
	outFile, err := os.Create(outputBin)
	if err != nil {
		fmt.Printf("❌ 创建输出文件失败: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	// 写入编译后的Go二进制
	_, _ = outFile.Write(tmpBinContent)
	// 写入分隔标记
	_, _ = outFile.Write([]byte("===LUA_PACK_ZIP_START==="))
	// 写入zip压缩的项目所有文件
	_, _ = outFile.Write(zipBuf.Bytes())

	fmt.Printf("✅ 打包成功！输出产物: %s\n", outputBin)
}
