// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ioutil

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Random number state.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.

// 随机数状态。
// 我们生成随机的临时文件名来给还不存在的文件一个好机会 —— 保持尝试 TempFile 的次数最小。
var rand uint32
var randmu sync.Mutex

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextSuffix() string {
	randmu.Lock()
	r := rand
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes // 此常数来源于《数值分析》
	rand = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

// TempFile creates a new temporary file in the directory dir
// with a name beginning with prefix, opens the file for reading
// and writing, and returns the resulting *os.File.
// If dir is the empty string, TempFile uses the default directory
// for temporary files (see os.TempDir).
// Multiple programs calling TempFile simultaneously
// will not choose the same file.  The caller can use f.Name()
// to find the pathname of the file.  It is the caller's responsibility
// to remove the file when no longer needed.

// TempFile 在目录 dir 中创建一个名字以 prefix 开头的新的临时文件，打开该文件以用于读写，
// 并返回其结果 *os.File。若 dir 为空字符串，TempFile 就会为临时文件使用默认的目录（见
// os.TempDir）。多程序同时调用 TempFile 将不会选择相同的文件。调用者可使用 f.Name()
// 来查找该文件的路径名 pathname。当该文件不再被需要时，调用者应负责将其移除。
func TempFile(dir, prefix string) (f *os.File, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	nconflict := 0
	for i := 0; i < 10000; i++ {
		name := filepath.Join(dir, prefix+nextSuffix())
		f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				rand = reseed()
			}
			continue
		}
		break
	}
	return
}

// TempDir creates a new temporary directory in the directory dir
// with a name beginning with prefix and returns the path of the
// new directory.  If dir is the empty string, TempDir uses the
// default directory for temporary files (see os.TempDir).
// Multiple programs calling TempDir simultaneously
// will not choose the same directory.  It is the caller's responsibility
// to remove the directory when no longer needed.

// TempDir 在目录 dir 中创建一个名字以 prefix 开头的新的临时目录并返回该新目录的路径。
// 若 dir 为空字符串，TempDir 就会为临时文件（Unix将目录也视作文件）使用默认的目录（见
// os.TempDir）。多程序同时调用 TempDir 将不会选择相同的目录。当该目录不再被需要时，
// 调用者应负责将其移除。
func TempDir(dir, prefix string) (name string, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	nconflict := 0
	for i := 0; i < 10000; i++ {
		try := filepath.Join(dir, prefix+nextSuffix())
		err = os.Mkdir(try, 0700)
		if os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				rand = reseed()
			}
			continue
		}
		if err == nil {
			name = try
		}
		break
	}
	return
}
