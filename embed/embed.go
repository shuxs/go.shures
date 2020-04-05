package embed

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/shuxs/go.shures/res"

	"golang.org/x/tools/imports"
)

type Filter func(f *res.File) bool

func Process(dir, pkg, varName string, out string, filter filepath.WalkFunc, noDependent bool) (err error) {
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

	dirs, files, err := getFiles(dir, filter, 5)
	if err != nil {
		return err
	}

	typePrefix := strings.ToLower(varName[:1]) + varName[1:]
	data := map[string]interface{}{
		"Package": pkg,
		"Name":    varName,
		"Folders": dirs,
		"FileMap": files,

		"VarDirs":      fmt.Sprintf("%sDirs", typePrefix),
		"VarFiles":     fmt.Sprintf("%sFiles", typePrefix),
		"FSType":       fmt.Sprintf("%sFS", typePrefix),
		"FileType":     fmt.Sprintf("%sFile", typePrefix),
		"HttpFileType": fmt.Sprintf("%sHttpFile", typePrefix),
		"FileMapType":  fmt.Sprintf("%sFileMap", typePrefix),
		"DirMapType":   fmt.Sprintf("%sDirMap", typePrefix),
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

func getFiles(root string, filter filepath.WalkFunc, maxDepth int) ([]*res.File, map[string][]*res.File, error) {
	root, err := filepath.Abs(filepath.FromSlash(root))
	if err != nil {
		return nil, nil, err
	}

	var (
		dirs  = make([]*res.File, 0)
		files = make(map[string][]*res.File)
	)

	//判断传入路径是文件夹还是文件
	stat, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}

	//如果是文件直接返回
	if !stat.IsDir() {
		data, err := ioutil.ReadFile(root)
		if err != nil {
			return nil, nil, fmt.Errorf("readAll return err: %w", err)
		}

		dirs = []*res.File{{
			Path:      "/",
			FileIsDir: true,
		}}

		name := stat.Name()
		files["/"] = []*res.File{{
			Path:        "/" + name,
			Compressed:  encode(data),
			FileName:    name,
			FileIsDir:   false,
			FileSize:    int(stat.Size()),
			FileModTime: stat.ModTime().Unix(),
		}}
		return dirs, files, nil
	}

	//文件夹，遍历
	err = filepath.Walk(root, func(path string, info os.FileInfo, ex error) error {
		if ex != nil {
			return ex
		}

		//计算相对路径
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if relPath == "" || relPath == "." {
			relPath = ""
		}

		//应用过滤器
		if err := filter(relPath, info, err); err != nil {
			return err
		}

		//计算深度
		if maxDepth > 0 && info.IsDir() {
			if depth := strings.Count(strings.Trim(relPath, string(filepath.Separator)), string(filepath.Separator)); depth > maxDepth {
				return filepath.SkipDir
			}
		}

		//资源文件对象
		rFile := &res.File{
			Path:        "/" + filepath.ToSlash(relPath), //路径格式归一
			FileName:    info.Name(),
			FileIsDir:   info.IsDir(),
			FileSize:    int(info.Size()),
			FileModTime: info.ModTime().Unix(),
		}

		if rFile.Path == "/" {
			rFile.FileName = ""
		}

		//如果是文件夹，处理完毕
		if rFile.FileIsDir {
			dirs = append(dirs, rFile)
			return nil
		}

		//是文件，读取，编码
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("readAll return err: %w", err)
		}
		rFile.Compressed = encode(data)

		d := filepath.Dir(rFile.Path)
		files[d] = append(files[d], rFile)
		return nil
	})

	return dirs, files, err
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
	defer doClose(gzw)
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
	for {
		n, _ := r.Read(chunk)
		if n == 0 {
			break
		}
		w.Write(chunk[:n])
		w.WriteRune('\n')
	}

	return w.String()
}

func doClose(closer io.Closer) {
	_ = closer.Close()
}
