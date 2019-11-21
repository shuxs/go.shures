package embed

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/shuxs/go.shures/res"
	"golang.org/x/tools/imports"
)

type Filter func(f *res.File) bool

func Process(dir, pkg, varName string, out string, filter Filter, noDependent bool) (err error) {
	defer func() {
		if rev := recover(); rev != nil {
			if _, ok := rev.(error); ok {
				err = rev.(error)
			}
		}
	}()

	var t *template.Template
	if noDependent {
		t = template.Must(template.New("").Parse(templateNoDependence))
	} else {
		t = template.Must(template.New("").Parse(templateDependenceRes))
	}

	dirs, files, err := GetFiles(dir, filter, 5)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"pkg":   pkg,
		"var":   varName,
		"dirs":  dirs,
		"files": files,
	}

	w := &bytes.Buffer{}
	if err = t.Execute(w, data); err != nil {
		return err
	}

	if err = ioutil.WriteFile(out, w.Bytes(), 0666); err != nil {
		return err
	}

	v, err := imports.Process(out, w.Bytes(), nil)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(out, v, 0666); err != nil {
		return err
	}

	return nil
}

func GetFiles(path string, filter Filter, maxDepth int) ([]*res.File, map[string][]*res.File, error) {
	var (
		dirs  = make([]*res.File, 0, 20)
		files = make(map[string][]*res.File, 100)
		err   = fill(path, path, &dirs, &files, filter, 0, maxDepth)
	)
	return dirs, files, err
}

func fill(prefix, path string, dirs *[]*res.File, files *map[string][]*res.File, filter Filter, depth, maxDepth int) error {
	if maxDepth > 1 && depth >= maxDepth {
		return nil
	}

	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	pathName := filepath.ToSlash(path[len(prefix):])
	if pathName == "" {
		pathName = "/"
	}
	f := &res.File{
		Path:        pathName,
		FileName:    fi.Name(),
		FileIsDir:   fi.IsDir(),
		FileSize:    int(fi.Size()),
		FileModTime: fi.ModTime().Unix(),
	}

	if f.Path == "/" {
		f.FileName = ""
	}

	if filter != nil && !filter(f) {
		return nil
	}

	if f.FileIsDir {
		*dirs = append(*dirs, f)
		osf, err := os.Open(path)
		if err != nil {
			return err
		}
		names, err := osf.Readdirnames(-1)
		if err != nil {
			return err
		}
		for _, name := range names {
			childPath := filepath.Join(path, name)
			if err := fill(prefix, childPath, dirs, files, filter, depth+1, maxDepth); err != nil {
				return err
			}
		}
		return nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readAll return err: %w", err)
	}
	f.Compressed = encode(data)

	d := filepath.Dir(pathName)
	fs := append((*files)[d], f)
	sort.Slice((*files)[d], func(i, j int) bool { return strings.Compare(fs[i].FileName, fs[j].FileName) == -1 })
	(*files)[d] = fs
	return nil
}

func encode(data []byte) string {
	data, err := gzipCompress(data)
	if err != nil {
		log.Fatal(err)
	}
	return chunkBase64Encode(data, 90)
}

func gzipCompress(data []byte) ([]byte, error) {
	w := &bytes.Buffer{}
	gzw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	defer gzw.Close()
	if _, err = gzw.Write(data); err != nil {
		return nil, err
	}
	if err = gzw.Flush(); err != nil {
		return w.Bytes(), err
	}
	return w.Bytes(), nil
}

func chunkBase64Encode(data []byte, chunkSize int) string {
	v := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	if base64.StdEncoding.Encode(v, data); len(v) < chunkSize {
		return string(v)
	}

	w := strings.Builder{}
	w.WriteRune('\n')

	chunk := make([]byte, chunkSize)
	r := bytes.NewReader(v)
	for n, _ := r.Read(chunk); n > 0; n, _ = r.Read(chunk) {
		w.Write(chunk[:n])
		w.WriteRune('\n')
	}

	return w.String()
}
