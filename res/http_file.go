package res

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

var _ http.File = &httpFile{}

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

	fis, find := f.Dirs[f.Path]
	if !find {
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

