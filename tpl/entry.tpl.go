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
	// 创建临时目录，程序退出时自动清理所有临时文件
	tmpDir, err := os.MkdirTemp("", "lua-pack-*")
	if err != nil {
		panic(fmt.Sprintf("创建临时目录失败: %v", err))
	}
	defer os.RemoveAll(tmpDir)

	// 获取当前运行的二进制程序自身路径
	exePath, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("获取程序路径失败: %v", err))
	}
	f, err := os.Open(exePath)
	if err != nil {
		panic(fmt.Sprintf("打开程序文件失败: %v", err))
	}
	defer f.Close()

	// 读取二进制完整内容，分割出zip数据区
	content, err := io.ReadAll(f)
	if err != nil {
		panic(fmt.Sprintf("读取程序内容失败: %v", err))
	}

	// 查找zip数据起始标记，分离Go编译的二进制 + zip压缩数据
	zipMarker := []byte("===LUA_PACK_ZIP_START===")
	markerIdx := bytes.Index(content, zipMarker)
	if markerIdx == -1 {
		panic("未找到嵌入的项目压缩包数据，打包产物异常！")
	}
	zipData := content[markerIdx+len(zipMarker):]

	// 将zip二进制数据写入临时压缩包
	zipTmpFile := filepath.Join(tmpDir, "project.zip")
	if err := os.WriteFile(zipTmpFile, zipData, 0644); err != nil {
		panic(fmt.Sprintf("写入临时压缩包失败: %v", err))
	}

	// 解压所有项目文件到临时目录
	if err := unzip(zipTmpFile, tmpDir); err != nil {
		panic(fmt.Sprintf("解压项目文件失败: %v", err))
	}

	// 切换工作目录到临时目录，保证lua编译器能正确读取相对路径文件
	if err := os.Chdir(tmpDir); err != nil {
		panic(fmt.Sprintf("切换工作目录失败: %v", err))
	}

	// 构造运行命令：编译器路径 + lua入口 + 外部传入的所有参数
	compilerPath := "{{.CompilerPath}}"
	luaMain := "{{.LuaMainPath}}"
	// 兼容windows和linux/mac的路径分隔符差异
	if runtime.GOOS == "windows" {
		compilerPath = strings.ReplaceAll(compilerPath, "/", "\\")
		luaMain = strings.ReplaceAll(luaMain, "/", "\\")
	}

	// 拼接参数：编译器执行lua入口，透传用户启动时的所有参数
	cmdArgs := append([]string{luaMain}, os.Args[1:]...)
	cmd := exec.Command(compilerPath, cmdArgs...)
	// 继承所有标准输入输出，保证lua程序能正常交互/打印日志
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行lua编译器并退出
	if err := cmd.Run(); err != nil {
		panic(fmt.Sprintf("lua程序执行失败: %v", err))
	}
}

// unzip 通用解压函数，将zip文件解压到指定目录
func unzip(zipFile, destDir string) error {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(fpath), os.ModePerm)

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

		_, _ = io.Copy(outFile, inFile)
	}
	return nil
}
