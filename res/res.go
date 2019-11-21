package res

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

var _ http.FileSystem = &FS{}
var _ os.FileInfo = &File{}
var _ http.File = &httpFile{}

type FileMap map[string]*File
type DirMap map[string][]os.FileInfo

type FS struct {
	Files FileMap
	Dirs  DirMap
}

func (fs *FS) prepare(name string) (*File, error) {
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

func (fs *FS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File(fs)
}

type File struct {
	Path        string //文件路径
	Compressed  string //压缩编码后的字符串
	FileName    string //文件名
	FileIsDir   bool   //是否目录
	FileSize    int    //文件大小
	FileModTime int64  //文件修改时间

	ChildPaths []string
	once      sync.Once
	data      []byte //文件数据
}

func (f *File) Name() string {
	return f.FileName
}

func (f *File) Size() int64 {
	return int64(f.FileSize)
}

func (f *File) Mode() os.FileMode {
	return 0444
}

func (f *File) ModTime() time.Time {
	return time.Unix(f.FileModTime, 0)
}

func (f *File) IsDir() bool {
	return f.FileIsDir
}

func (f *File) Sys() interface{} {
	return f
}

func (f *File) File(fs *FS) (http.File, error) {
	return &httpFile{
		FS:     fs,
		File:   f,
		Reader: bytes.NewReader(f.data),
	}, nil
}

type httpFile struct {
	*FS
	*File
	*bytes.Reader
}

func (f *httpFile) Close() error {
	return nil
}

func (f *httpFile) Readdir(count int) ([]os.FileInfo, error) {
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

func (f *httpFile) Stat() (os.FileInfo, error) {
	return f.File, nil
}
