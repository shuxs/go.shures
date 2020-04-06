package embed

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/shuxs/go.shures/res"
	"golang.org/x/tools/imports"
)

func New() *Option {
	return &Option{MaxDepth: 10}
}

type Matcher func(path string, info os.FileInfo) error

type FileMap struct {
	Option   *Option
	Resource *res.Resource
	Children []*FileMap
}

type Option struct {
	SourcePath string  //源路径
	Package    string  //包名
	Name       string  //变量名
	Target     string  //目标文件名
	NoDep      bool    //是否依赖
	Matcher    Matcher //过滤器
	MaxDepth   int     //扫描目录最深
	Force      bool    //是否覆盖已存在的目标文件

	ResourceType string
}

func (o *Option) Setup() error {
	if o.SourcePath == "" {
		o.SourcePath = "."
	}

	fp, _ := filepath.Abs(o.SourcePath)
	stat, err := os.Stat(fp)
	if err != nil {
		return fmt.Errorf("源路径[ %s ]异常: %w", o.SourcePath, err)
	}

	if stat.IsDir() {
		fileName := filepath.Base(fp)
		folderName := stat.Name()

		if o.Name == "" {
			o.Name = hump(folderName)
		}

		if o.Target == "" {
			o.Target = filepath.Join(o.SourcePath, fileName+".go")
		}
	} else {
		fileName := filepath.Base(fp)
		if o.Name == "" {
			o.Name = hump(fileName)
		}

		if o.Target == "" {
			o.Target = o.SourcePath + ".go"
		}
	}

	target, _ := filepath.Abs(o.Target)
	if o.Package == "" {
		o.Package = underline(filepath.Base(filepath.Dir(target)))
	}

	if o.NoDep {
		o.ResourceType = fmt.Sprintf("%sResource", o.Name)
	} else {
		o.ResourceType = "res.Resource"
	}

	return nil
}

func (o *Option) ValidateTarget() (bool, error) {
	if o.Target == "-" {
		return true, nil
	}

	stat, err := os.Stat(o.Target)
	if err != nil {
		if os.IsNotExist(err) { //文件不存在，通过
			return true, nil
		}
		return false, fmt.Errorf("检查: 目标路径[ %s ]异常, %w", o.Target, err) //读取文件状态错误，不通过
	}

	//目标是目录，不通过
	if stat.IsDir() {
		return false, fmt.Errorf("检查: 目标[ %s ]是一个目录", o.Target)
	}

	//文件存在，如果不是 force, 不通过
	return o.Force, nil
}

func (o *Option) Process(debug bool) (data string, err error) {
	defer func() {
		if rev := recover(); rev != nil {
			if e, ok := rev.(error); ok {
				err = fmt.Errorf("未处理的错误: %w", e)
			} else {
				err = fmt.Errorf("未处理的错误: %v", rev)
			}
		}
	}()

	fMap, err := o.GetFiles()
	if err != nil {
		return "", fmt.Errorf("生成: 编码文件出错, %w", err)
	}

	w := &bytes.Buffer{}
	if err = template.Must(template.New("").Parse(goTemplate)).ExecuteTemplate(w, "go", fMap); err != nil {
		return "", fmt.Errorf("生成: 执行模板出错, %w", err)
	}

	v, err := imports.Process("", w.Bytes(), nil)
	if err != nil {
		if !debug {
			return "", fmt.Errorf("生成: 格式化出错, %w", err)
		}
		v = w.Bytes()
	}

	if o.Target != "-" {
		if err = ioutil.WriteFile(o.Target, v, 0666); err != nil {
			return "", fmt.Errorf("生成: 写入文件出错, %w", err)
		}
	}

	return string(v), nil
}

func (o *Option) GetFiles() (fMap *FileMap, err error) {
	matcher := func(path string, info os.FileInfo) error {
		if path == o.Target {
			return filepath.SkipDir
		}
		if err := o.Matcher(path, info); err != nil {
			return err
		}
		return nil
	}

	fMap, err = o.getFiles(o.SourcePath, "", matcher, 0)
	return
}

func (o *Option) getFiles(root string, name string, matcher Matcher, depth int) (*FileMap, error) {
	fullPath := filepath.Join(root, name)

	//判断传入路径是文件夹还是文件
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	if err := matcher(filepath.ToSlash(fullPath), stat); err != nil {
		return nil, err
	}

	fMap := &FileMap{
		Option: o,
		Resource: &res.Resource{
			FileName:    name,
			FileIsDir:   stat.IsDir(),
			FileModTime: stat.ModTime().Unix(),
		},
	}

	//如果是文件直接返回
	if !fMap.Resource.FileIsDir {
		fMap.Resource.FileSize = stat.Size()
		fMap.Resource.Compressed, err = encode(fullPath)
		return fMap, err
	}

	if depth+1 < o.MaxDepth {
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}

		children, err := f.Readdirnames(200)
		if err != nil {
			return nil, err
		}
		_ = f.Close()

		fmt.Println(children)
		fMap.Children = make([]*FileMap, 0, len(children))
		for _, c := range children {
			cMap, err := o.getFiles(fullPath, c, matcher, depth+1)
			if err != nil {
				if err == filepath.SkipDir {
					continue
				}
				return nil, err
			}
			fMap.Children = append(fMap.Children, cMap)
			//fMap.Resource.Files[c] = cMap.Resource
		}
	}

	return fMap, nil
}

func encode(fn string) (string, error) {
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", err
	}
	if data, err = gzipCompress(data); err != nil {
		return "", err
	}
	return chunkBase64Encode(data, 64), nil
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

//驼峰命令
func hump(src string) string {
	var (
		out    = make([]rune, 0, len(src))
		needUp = true
	)
	for _, n := range src {
		if ('A' <= n && n <= 'Z') || ('a' <= n && n <= 'z') {
			if needUp && 'a' <= n && n <= 'z' {
				needUp = false
				n -= 'a' - 'A'
			}
			out = append(out, n)
		} else {
			needUp = true
		}
	}
	return string(out)
}

//下划线命令
func underline(src string) string {
	var (
		out    = make([]rune, 0, len(src))
		needUp = false
		lastUp = false
	)
	for _, n := range src {
		if ('A' <= n && n <= 'Z') || ('a' <= n && n <= 'z') {
			if 'A' <= n && n <= 'Z' {
				n += 'a' - 'A'
				if !lastUp {
					needUp = true
				}
			}
			if needUp {
				lastUp = true
				needUp = false
				out = append(out, '_')
			}
			out = append(out, n)
		} else {
			if !lastUp {
				needUp = true
			}
		}
	}
	return string(out)
}

const goTemplate = `
{{ define "go" -}}

// Code generated by shures; DO NOT EDIT.
package {{ .Option.Package }}

{{ if .Option.NoDep -}}
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

/* define */

type {{ .Option.ResourceType }} struct {
    Compressed  string
    FileName    string
    FileIsDir   bool
    FileSize    int64
    FileModTime int64
	FileMap     map[string]int
	Files       []*{{ .Option.ResourceType }}

    reader io.ReadSeeker
    data   []byte
    err    error
    once   sync.Once
}

func (f *{{ .Option.ResourceType }}) Close() error {
    return nil
}

func (f *{{ .Option.ResourceType }}) Read(p []byte) (n int, err error) {
    f.prepare()
    if f.reader != nil {
        return f.reader.Read(p)
    }
    return 0, io.EOF
}

func (f *{{ .Option.ResourceType }}) Seek(offset int64, whence int) (int64, error) {
    f.prepare()
    return f.reader.Seek(offset, whence)
}

func (f *{{ .Option.ResourceType }}) Readdir(count int) ([]os.FileInfo, error) {
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

func (f *{{ .Option.ResourceType }}) Stat() (os.FileInfo, error) {
    f.prepare()
    return f, nil
}

func (f *{{ .Option.ResourceType }}) Name() string {
    return f.FileName
}

func (f *{{ .Option.ResourceType }}) Size() int64 {
    return f.FileSize
}

func (f *{{ .Option.ResourceType }}) Mode() os.FileMode {
    return 0444
}

func (f *{{ .Option.ResourceType }}) ModTime() time.Time {
    return time.Unix(f.FileModTime, 0)
}

func (f *{{ .Option.ResourceType }}) IsDir() bool {
    return f.FileIsDir
}

func (f *{{ .Option.ResourceType }}) Sys() interface{} {
    return f
}

func (f *{{ .Option.ResourceType }}) Bytes() ([]byte, error) {
    f.prepare()
    return f.data, f.err
}

func (f *{{ .Option.ResourceType }}) prepare() {
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

func (f *{{ .Option.ResourceType }}) Open(path string) (http.File, error) {
	f.prepare()
	separator := string(filepath.Separator)
	path = strings.Trim(path, separator)

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

{{- else -}}

import (
    "go.shu.run/go.shures/res"
)

{{- end }}

{{ if .Resource.FileIsDir -}}
var {{ .Option.Name }} = &{{ .Option.ResourceType }} {{ template "dir" . }}
{{- else -}}
var {{ .Option.Name }} = &{{ .Option.ResourceType }} {{ template "file" .Resource }}
{{- end }}

{{- end }}

{{ define "file" -}}
{
  FileName:    "{{ .FileName }}",
  FileSize:    {{ .FileSize }},
  FileModTime: {{ .FileModTime }},
  ` + "Compressed:  `{{ .Compressed }}`," + `
}
{{- end }}

{{ define "dir" -}}
{
  FileName:    "{{ .Resource.FileName }}",
  FileIsDir:   {{ .Resource.FileIsDir }},
  FileModTime: {{ .Resource.FileModTime }},
  Files: []*{{ .Option.ResourceType }}{
    {{ range $item := .Children -}}
	{{ if $item.Resource.FileIsDir -}}
		{{- template "dir" $item }}
    {{- else -}}
		{{- template "file" $item.Resource }}
    {{- end }},
    {{ end }}
  },
}
{{- end }}
`
