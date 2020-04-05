package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/shuxs/go.shures/embed"
	"github.com/spf13/pflag"
)

const version = "0.1.1"

func main() {
	var (
		dir, pkg, varName, out string
		excludes, includes     []string
		noDependent            bool
		v                      bool
		help                   bool
	)

	pflag.CommandLine.SortFlags = false
	pflag.ErrHelp = errors.New("")

	pflag.Usage = func() {
		name, _ := os.Executable()
		_, name = filepath.Split(name)
		_, _ = fmt.Fprintf(os.Stderr, `%s [OPTIONS] filename/dirname

%s

Options:
%s
`,
			name,
			"Embed files into a Go executable",
			pflag.CommandLine.FlagUsages())
	}

	pflag.StringVarP(&pkg, "pkg", "p", "", "包名")
	pflag.StringVar(&varName, "var", "", "变量名")
	pflag.StringVarP(&out, "out", "o", "", "保存文件")
	pflag.StringArrayVarP(&excludes, "exclude", "e", nil, "排除(正则表达式)")
	pflag.StringArrayVarP(&includes, "include", "i", nil, "包含(正则表达式)")
	pflag.BoolVar(&noDependent, "no-dep", false, "是否无依赖(github.com/shuxs/go.shures/res)")
	pflag.BoolVarP(&v, "version", "V", false, "版本号")
	pflag.BoolVarP(&help, "help", "h", false, "打印使用方法")
	pflag.Parse()

	if help {
		pflag.Usage()
		return
	}

	if v {
		fmt.Printf("%s v%s, build with %s\n", os.Args[0], version, runtime.Version())
		return
	}

	if len(pflag.Args()) > 0 {
		dir, _ = filepath.Abs(pflag.Args()[0])
	}
	if dir == "" {
		pflag.Usage()
		os.Exit(-1)
		return
	}
	name := filepath.Base(dir)
	if out == "" {
		out = filepath.Join(dir, name+".go")
	}
	if varName == "" {
		varName = strings.Title(name)
	}
	if pkg == "" {
		pkg = name
	}

	fmt.Println("嵌入目录:", dir)
	if pkg != "" {
		fmt.Println("指定包名:", pkg)
	}
	if varName != "" {
		fmt.Println("资源变量:", varName)
	}
	if out != "" {
		fmt.Println("输出文件:", out)
	}
	if noDependent {
		fmt.Println("外部依赖: 否")
	}

	var includeRegs []*regexp.Regexp
	var excludeRegs []*regexp.Regexp

	if len(includes) > 0 {
		fmt.Println("包含条件:")
		for _, include := range includes {
			fmt.Printf("  %s\n", include)
			r, err := regexp.Compile(include)
			if err != nil {
				log.Fatal(err)
			}
			includeRegs = append(includeRegs, r)
		}
	}

	if len(excludes) > 0 {
		fmt.Println("过滤条件:")
		for _, exclude := range excludes {
			fmt.Printf("  %s\n", exclude)
			r, err := regexp.Compile(exclude)
			if err != nil {
				log.Fatal(err)
			}
			excludeRegs = append(excludeRegs, r)
		}
	}

	filter := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		for _, r := range includeRegs {
			if r.MatchString(path) {
				return nil
			}
		}
		for _, r := range excludeRegs {
			if r.MatchString(path) {
				return filepath.SkipDir
			}
		}
		return nil
	}

	if err := embed.Process(dir, pkg, varName, out, filter, noDependent); err != nil {
		log.Fatal(err)
	}
}
