package res

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var _ os.FileInfo = (*Resource)(nil)
var _ http.File = (*Resource)(nil)
var _ http.FileSystem = (*Resource)(nil)

type Resource struct {
	Compressed  string         //压缩编码后的字符串
	FileName    string         //文件名
	FileIsDir   bool           //是否目录
	FileSize    int64          //文件大小
	FileModTime int64          //文件修改时间
	FileMap     map[string]int //下一级
	Files       []*Resource    //下一级

	reader io.ReadSeeker //数据读取器
	data   []byte        //已解压的数据
	err    error         //解压错误
	once   sync.Once
}

/* http.Resource */

func (f *Resource) Close() error {
	return nil
}

func (f *Resource) Read(p []byte) (n int, err error) {
	f.prepare()
	if f.reader != nil {
		return f.reader.Read(p)
	}
	return 0, io.EOF
}

func (f *Resource) Seek(offset int64, whence int) (int64, error) {
	f.prepare()
	return f.reader.Seek(offset, whence)
}

func (f *Resource) Readdir(count int) ([]os.FileInfo, error) {
	if !f.FileIsDir {
		return nil, os.ErrPermission
	}

	f.prepare()
	fis := make([]os.FileInfo, 0, count)
	for i := 0; i < count && i < len(f.Files); i++ {
		fis = append(fis, f.Files[i])
	}
	return fis, nil
}

func (f *Resource) Stat() (os.FileInfo, error) {
	f.prepare()
	return f, nil
}

/* os.FileInfo */

func (f *Resource) Name() string {
	return f.FileName
}

func (f *Resource) Size() int64 {
	return f.FileSize
}

func (f *Resource) Mode() os.FileMode {
	return 0444
}

func (f *Resource) ModTime() time.Time {
	return time.Unix(f.FileModTime, 0)
}

func (f *Resource) IsDir() bool {
	return f.FileIsDir
}

func (f *Resource) Sys() interface{} {
	return f
}

func (f *Resource) Bytes() ([]byte, error) {
	f.prepare()
	return f.data, f.err
}

func (f *Resource) prepare() {
	f.once.Do(func() {
		if !f.FileIsDir && f.FileSize > 0 {
			reader, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.Compressed)))
			if err != nil {
				f.err = err
				return
			}
			defer func() {
				_ = reader.Close()
			}()

			f.data, f.err = ioutil.ReadAll(reader)
			if f.err == nil {
				f.reader = bytes.NewReader(f.data)
			}
			return
		}

		sort.Slice(f.Files, func(i, j int) bool {
			return strings.Compare(f.Files[i].FileName, f.Files[j].FileName) > 0
		})

		if f.FileIsDir && len(f.Files) > 0 {
			f.FileMap = make(map[string]int, len(f.Files))
			for i, item := range f.Files {
				f.FileMap[item.FileName] = i
			}
		}
	})
}

/* http.FileSystem */

func (f *Resource) Open(path string) (http.File, error) {
	f.prepare()
	separator := string(filepath.Separator)
	path = strings.Trim(path, separator)

	//多级目录
	if i := strings.Index(path, separator); i != -1 {
		if !f.IsDir() || f.Files == nil {
			return nil, os.ErrNotExist
		}
		name := path[:i]
		path = path[i+1:]

		if now, find := f.FileMap[name]; find {
			return f.Files[now].Open(path)
		}
		return nil, os.ErrNotExist
	}

	if f.FileName != path {
		return nil, os.ErrNotExist
	}

	return f, nil
}
