package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/shuxs/go.shures/embed"
	"github.com/spf13/pflag"
)

const version = "1.0.0"

var flag *pflag.FlagSet

func init() {
	pflag.ErrHelp = errors.New("")

	name, _ := os.Executable()
	name = filepath.Base(name)

	flag = pflag.NewFlagSet(name, pflag.ExitOnError)
	flag.SortFlags = false
	flag.Usage = func() {
		fmt.Printf("%s version %s build with %s\n", name, version, runtime.Version())
		fmt.Println("Embed files into a Go executable")
		fmt.Println()
		fmt.Printf("Usage: %s [OPTIONS] filename/dirname\n", name)
		fmt.Println()
		fmt.Printf("Options:\n%s\n", flag.FlagUsages())
	}
}

func main() {
	var (
		help  bool
		debug bool
		mExps []string //筛选(正则表达式)
		ember = embed.New()
	)

	flag.StringVar(&ember.Package, "pkg", "", "输出的包名")
	flag.StringVar(&ember.Name, "name", "", "输出的变量名称")
	flag.BoolVar(&ember.NoDep, "no-dep", false, "是否生成不带依赖资源代码 (不需要 import github.com/shuxs/go.shures/res)")
	flag.StringVarP(&ember.Target, "out", "o", "", "输出文件路径")
	flag.BoolVarP(&ember.Force, "force", "f", false, "是否覆盖目标文件")
	flag.StringArrayVarP(&mExps, "match", "m", nil, "筛选(正则表达式)")
	flag.BoolVarP(&debug, "debug", "d", debug, "调试模式")
	flag.BoolVarP(&help, "help", "h", help, "打印使用方法")

	fatalIf(flag.Parse(os.Args[1:]))

	if help || flag.NArg() == 0 {
		flag.Usage()
		return
	}

	ember.SourcePath = flag.Arg(0)
	ember.Matcher = getMatcher(mExps...)
	initialize(ember)

	output, err := ember.Process(debug)
	if output != "" && (err != nil || ember.Target == "-") {
		fmt.Println()
		fmt.Println(output)
		fmt.Println()
	}
	fatalIf(err)
}

func initialize(ember *embed.Option) {
	fatalIf(ember.Setup())
	for {
		ok, err := ember.ValidateTarget()
		fatalIf(err)
		if ok {
			break
		}

		fmt.Printf("目标文件[ %s ]已存在，是否覆盖: Y or n: ", ember.Target)
		var forces string
		_, _ = fmt.Scanln(&forces)
		forces = strings.ToUpper(forces)

		if forces != "" {
			if forces == "N" {
				os.Exit(0)
			}
			if forces == "Y" {
				ember.Force = true
			}
		}
	}
}

func getMatcher(mExps ...string) func(path string, info os.FileInfo) error {
	var mRegs []*regexp.Regexp
	if len(mExps) > 0 {
		//fmt.Println("文件匹配:")
		for _, filterExp := range mExps {
			//fmt.Printf("  %s\n", filterExp)
			mReg, err := regexp.Compile(filterExp)
			fatalIf(err)
			mRegs = append(mRegs, mReg)
		}
	}

	return func(path string, info os.FileInfo) error {
		if len(mRegs) > 0 {
			for _, mReg := range mRegs {
				if mReg.MatchString(path) {
					return nil
				}
			}
			return filepath.SkipDir
		}
		return nil
	}
}

func fatalIf(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
