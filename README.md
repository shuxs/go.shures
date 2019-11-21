# shures

> files into a Go executable \
> only export one http.FileSystem variable.

## install

```shell script
go get -u -v github.com/shuxs/go.shures/shures
```

## usage

```shell script
$ shures --help
shures [OPTIONS] filename/dirname

Embed files into a Go executable

Options:
  -p, --pkg string        包名
      --var string        变量名
  -o, --out string        保存文件
  -e, --exclude strings   排除(正则表达式)
  -i, --include strings   包含(正则表达式)
      --no-dependent      是否无依赖(github.com/shuxs/go.shures/res)
  -V, --version           版本号
  -h, --help              打印使用方法
```

## example #1

```shell script
$ shures ./assets
嵌入目录: ./assets
指定包名: assets
资源变量: Assets
输出文件: ./assets/assets.go
是否依赖: true
过滤条件:
包含条件:
```

```shell script
cat ./assets/assets.go
```
```go
// Code generated by shures; DO NOT EDIT.
package assets

import (
	"github.com/shuxs/go.shures/res"
)

var Assets = &res.FS{
	Files: _AssetsFiles,
	Dirs:  _AssetsDirs,
}

var _AssetsFiles = res.FileMap{

	"/": {
		Path:      "/",
		FileName:  "",
		FileIsDir: true,
	},

	"/hello.txt": {
		Path:        "/hello.txt",
		FileName:    "hello.txt",
		FileSize:    22,
		FileModTime: 1574321639,
		Compressed:  `H4sIAAAAAAAC/8pUz1VIVMhIzcnJVyipKFFIy8xJ1eMCAAAA//8=`,
	},
}

var _AssetsDirs = res.DirMap{
	"/": {
		_AssetsFiles["/hello.txt"],
	},
}
```

Using

```go
var fs http.FileSystem  = assets.Assets
//... code yourself
```

## example #2

```shell script
$ shures --no-dependent ./assets 
嵌入目录: ./assets
指定包名: assets
资源变量: Assets
输出文件: ./assets/assets.go
是否依赖: false
过滤条件:
包含条件:
```

```go
// cat assets/assets.go

// Code generated by shures; DO NOT EDIT.
package assets

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

var (
	_ http.FileSystem = &_AssetsFS{}
	_ os.FileInfo     = &_AssetsFile{}
	_ http.File       = &_AssetsHttpFile{}
)

type (
	_AssetsFileMap map[string]*_AssetsFile
	_AssetsDirMap  map[string][]os.FileInfo

	_AssetsHttpFile struct {
		*_AssetsFS
		*_AssetsFile
		*bytes.Reader
	}

	_AssetsFS struct {
		Files _AssetsFileMap
		Dirs  _AssetsDirMap
	}

	_AssetsFile struct {
		Path        string //文件路径
		Compressed  string //压缩编码后的字符串
		FileName    string //文件名
		FileIsDir   bool   //是否目录
		FileSize    int    //文件大小
		FileModTime int64  //文件修改时间

		ChildPaths []string
		once       sync.Once
		data       []byte //文件数据
	}
)

func (fs *_AssetsFS) prepare(name string) (*_AssetsFile, error) {
	f, present := fs.Files[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	f.once.Do(func() {
		if f.FileSize == 0 {
			return
		}
		var gr *gzip.Reader
		var bReader = base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.Compressed))
		if gr, err = gzip.NewReader(bReader); err != nil {
			return
		}
		f.data, _ = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs *_AssetsFS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File(fs)
}

func (f *_AssetsFile) Name() string {
	return f.FileName
}

func (f *_AssetsFile) Size() int64 {
	return int64(f.FileSize)
}

func (f *_AssetsFile) Mode() os.FileMode {
	return 0444
}

func (f *_AssetsFile) ModTime() time.Time {
	return time.Unix(f.FileModTime, 0)
}

func (f *_AssetsFile) IsDir() bool {
	return f.FileIsDir
}

func (f *_AssetsFile) Sys() interface{} {
	return f
}

func (f *_AssetsFile) File(fs *_AssetsFS) (http.File, error) {
	return &_AssetsHttpFile{
		_AssetsFS:   fs,
		_AssetsFile: f,
		Reader:      bytes.NewReader(f.data),
	}, nil
}

func (f *_AssetsHttpFile) Close() error {
	return nil
}

func (f *_AssetsHttpFile) Readdir(count int) ([]os.FileInfo, error) {
	if !f.FileIsDir {
		return nil, fmt.Errorf("go.res.Readdir: '%s' is not directory", f.FileName)
	}

	fis, ok := f.Dirs[f.Path]
	if !ok {
		return nil, fmt.Errorf("go.res.Readdir: '%s' is directory, but we have no info about content of this dir, local=%s", f.FileName, f.Path)
	}
	limit := count
	if count <= 0 || limit > len(fis) {
		limit = len(fis)
	}

	if len(fis) == 0 && count > 0 {
		return nil, io.EOF
	}

	return fis[0:limit], nil
}

func (f *_AssetsHttpFile) Stat() (os.FileInfo, error) {
	return f._AssetsFile, nil
}

var Assets = &_AssetsFS{
	Files: _AssetsFiles,
	Dirs:  _AssetsDirs,
}

var _AssetsFiles = _AssetsFileMap{
	"/": {
		Path:      "/",
		FileName:  "",
		FileIsDir: true,
	},
	"/hello.txt": {
		Path:        "/hello.txt",
		FileName:    "hello.txt",
		FileSize:    22,
		FileModTime: 1574321639,
		Compressed:  `H4sIAAAAAAAC/8pUz1VIVMhIzcnJVyipKFFIy8xJ1eMCAAAA//8=`,
	},
}

var _AssetsDirs = _AssetsDirMap{
	"/": {
		_AssetsFiles["/hello.txt"],
	},
}
```

## Other Choice

https://github.com/mjibson/esc