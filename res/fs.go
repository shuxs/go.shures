package res

import (
	"bytes"
	"net/http"
	"os"
	"path"
)

var _ http.FileSystem = &FS{}

type FS struct {
	Files FileMap
	Dirs  DirMap
}

func (fs *FS) Open(name string) (http.File, error) {
	f, find := fs.Files[path.Clean(name)]
	if !find {
		return nil, os.ErrNotExist
	}

	if f.prepare(); f.err != nil {
		return nil, f.err
	}

	return &httpFile{
		FS:     fs,
		File:   f,
		Reader: bytes.NewReader(f.data),
	}, nil
}
