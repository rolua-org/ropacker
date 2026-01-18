// 运行时入口程序的模板内容，被 pack.go 通过 embed 加载
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

	// 切换工作目录到临时目录，保证lua编译器能正确读取相对路径的lua文件
	if err := os.Chdir(tmpDir); err != nil {
		panic(fmt.Sprintf("切换工作目录失败: %v", err))
	}

	// ========== ✅ 核心修复开始 ==========
	// 1. 读取模板注入的相对路径编译器名、lua入口名
	compilerRelPath := "{{.CompilerPath}}"
	luaMain := "{{.LuaMainPath}}"

	// 2. 修复核心：拼接 临时目录+编译器相对路径 = 编译器绝对路径
	// 无论怎么切换工作目录，绝对路径永远能精准找到文件，彻底解决PATH寻址问题
	compilerAbsPath := filepath.Join(tmpDir, compilerRelPath)

	// 3. 跨平台路径兼容：Linux/Mac <-> Windows 路径分隔符互转
	if runtime.GOOS == "windows" {
		compilerAbsPath = strings.ReplaceAll(compilerAbsPath, "/", "\\")
		luaMain = strings.ReplaceAll(luaMain, "/", "\\")
	}
	// ========== ✅ 核心修复结束 ==========

	// ========== ✅ 必加修复：添加可执行权限 ==========
	// Linux/Mac 系统中，解压后的二进制文件会丢失可执行权限，Windows无影响
	// 这是zip解压的特性，必须手动赋予+x权限，否则会报 permission denied
	if runtime.GOOS != "windows" {
		if err := os.Chmod(compilerAbsPath, 0755); err != nil {
			panic(fmt.Sprintf("赋予编译器可执行权限失败: %v | 路径: %s", err, compilerAbsPath))
		}
	}

	// 拼接参数：编译器执行lua入口，透传用户启动时的所有参数
	cmdArgs := append([]string{luaMain}, os.Args[1:]...)
	// ========== ✅ 最终修复：使用绝对路径调用编译器 ==========
	cmd := exec.Command(compilerAbsPath, cmdArgs...)

	// 继承所有标准输入输出，保证lua程序能正常交互/打印日志
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 执行lua编译器并退出
	if err := cmd.Run(); err != nil {
		panic(fmt.Sprintf("lua程序执行失败: %v | 编译器绝对路径: %s", err, compilerAbsPath))
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
