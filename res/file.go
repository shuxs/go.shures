package res

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

var _ os.FileInfo = &File{}

type File struct {
	Path        string //文件路径
	Compressed  string //压缩编码后的字符串
	FileName    string //文件名
	FileIsDir   bool   //是否目录
	FileSize    int    //文件大小
	FileModTime int64  //文件修改时间

	data     []byte     //文件数据
	prepared bool       //是否已经解压
	err      error      //解压错误
	locker   sync.Mutex //解压操作锁
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

func (f *File) Bytes() ([]byte, error) {
	f.prepare()
	return f.data, f.err
}

func (f *File) prepare() {
	f.locker.Lock()
	defer f.locker.Unlock()
	if !f.prepared {
		f.prepared = true
		if f.FileSize > 0 {
			gr, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.Compressed)))
			if err != nil {
				f.err = err
				return
			}
			f.data, f.err = ioutil.ReadAll(gr)
		}
	}
}
