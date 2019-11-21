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
	"github.com/shuxs/go.shures/res"
	"github.com/spf13/pflag"
)

const version = "0.1.0"

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
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] filename/dirname\n\n%s\n\nOptions:\n%s\n",
			os.Args[0],
			"Embed files into a Go executable",
			pflag.CommandLine.FlagUsages())
	}

	pflag.StringVarP(&pkg, "pkg", "p", "", "包名")
	pflag.StringVar(&varName, "var", "", "变量名")
	pflag.StringVarP(&out, "out", "o", "", "保存文件")
	pflag.StringSliceVarP(&excludes, "exclude", "e", nil, "排除(正则表达式)")
	pflag.StringSliceVarP(&includes, "include", "i", nil, "包含(正则表达式)")
	pflag.BoolVar(&noDependent, "no-dependent", false, "是否无依赖(github.com/shuxs/go.shures/res)")
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
	fmt.Println("指定包名:", pkg)
	fmt.Println("资源变量:", varName)
	fmt.Println("输出文件:", out)
	fmt.Println("是否依赖:", !noDependent)
	fmt.Println("过滤条件:")
	for _, exclude := range excludes {
		fmt.Printf("  %s\n", exclude)
	}
	fmt.Println("包含条件:")

	for _, include := range includes {
		fmt.Printf("  %s\n", include)
	}

	var excludeRegs []*regexp.Regexp
	for _, exclude := range excludes {
		r, err := regexp.Compile(exclude)
		if err != nil {
			log.Fatal(err)
		}
		excludeRegs = append(excludeRegs, r)
	}

	var includeRegs []*regexp.Regexp
	for _, include := range includes {
		r, err := regexp.Compile(include)
		if err != nil {
			log.Fatal(err)
		}
		includeRegs = append(includeRegs, r)
	}

	f := func(f *res.File) bool {
		for _, r := range includeRegs {
			if r.MatchString(f.Path) {
				return true
			}
		}
		for _, r := range excludeRegs {
			if r.MatchString(f.Path) {
				return false
			}
		}
		return true
	}

	if err := embed.Process(dir, pkg, varName, out, f, noDependent); err != nil {
		log.Fatal(err)
	}
}
