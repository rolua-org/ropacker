package main

import (
	"archive/zip"
	_ "embed"
	"encoding/binary"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

const bootfn string = "boot.tpl.go"

//go:embed tpl/boot.tpl.go
var boottpl string

func pack(projectDir, compilerName, luaMain, outputBin string) {
	fmt.Println("ropacker 将会打包", projectDir, "内的所有文件, 并使用", compilerName, "作为解释器, 打包产物为", outputBin, ", 运行时将执行", luaMain, "内的代码")

	fmt.Println("切换临时工作目录...")

	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("can not get current dir: %v", err))
	}

	tmp, err := os.MkdirTemp(os.TempDir(), "ropacker-job-*")
	if err != nil {
		panic(fmt.Errorf("can not create temp dir: %v", err))
	}

	defer os.RemoveAll(tmp)

	chdir(tmp)

	fmt.Println("编译启动器...")

	bootl, err := template.New("boot").Parse(boottpl)
	if err != nil {
		panic(fmt.Errorf("can not parse tpl: %v", err))
	}

	bootf, err := os.Create(bootfn)
	if err != nil {
		panic(fmt.Errorf("can not create boot: %v", err))
	}

	defer bootf.Close()

	bootl.Execute(bootf, map[string]string{
		"compiler": compilerName,
		"lua":      luaMain,
	})

	run("go", "build", "-ldflags=-s -w -extldflags=-static", "-o", "boot", bootfn)

	fmt.Println("复制项目文件...")

	proj := filepath.Join(cwd, projectDir)
	compress(proj, "proj")

	fmt.Println("生成最终产物...")

	chdir(cwd)

	dist, err := os.Create(outputBin)
	if err != nil {
		panic(fmt.Errorf("can not create dist: %v", err))
	}

	defer dist.Close()

	bootp := filepath.Join(tmp, "boot")
	projp := filepath.Join(tmp, "proj")

	appendf(dist, bootp)
	appendf(dist, projp)

	booti, err := os.Stat(bootp)
	if err != nil {
		panic(fmt.Errorf("can not get boot info: %v", err))
	}

	bootlen := booti.Size()
	lenbuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenbuf, uint64(bootlen))

	if _, err := dist.Write(lenbuf); err != nil {
		panic(fmt.Errorf("can not write size suffix: %v", err))
	}

	dist.Sync()
	os.Chmod(outputBin, 0755)

	fmt.Println("打包完成")
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

func compress(dir, out string) {
	outf, err := os.Create(out)
	if err != nil {
		panic(fmt.Errorf("can not create zip: %v", err))
	}

	defer outf.Close()

	zipW := zip.NewWriter(outf)

	err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		dst, err := zipW.Create(rel)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(dst, src)
		src.Close()

		return copyErr
	})

	if err != nil {
		panic(fmt.Errorf("can not walk/write zip: %v", err))
	}

	if err := zipW.Close(); err != nil {
		panic(fmt.Errorf("can not close zip: %v", err))
	}
}

func appendf(f *os.File, src string) {
	bin, err := os.Open(src)
	if err != nil {
		panic(fmt.Errorf("can not open file: %v", err))
	}

	defer bin.Close()

	if _, err := io.Copy(f, bin); err != nil {
		panic(fmt.Errorf("copy error: %v", err))
	}
}
