package res

import (
	"os"
)

type FileMap map[string]*File
type DirMap map[string][]os.FileInfo
