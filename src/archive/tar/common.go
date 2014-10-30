// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tar implements access to tar archives.
// It aims to cover most of the variations, including those produced
// by GNU and BSD tars.
//
// References:
//   http://www.freebsd.org/cgi/man.cgi?query=tar&sektion=5
//   http://www.gnu.org/software/tar/manual/html_node/Standard.html
//   http://pubs.opengroup.org/onlinepubs/9699919799/utilities/pax.html

// Chensi Yuan: 2014-10-27 之所以第一行要改成点号“.”，
// 是由于godoc只识别点为模块索引结束符号
// 如果是全角句号“。”最终在 archive 索引页会显示整个注释

// tar包实现了tar格式压缩文件的存取.
// 本包目标是覆盖大多数tar的变种，包括GNU和BSD生成的tar文件。
//
// 参见：
//   http://www.freebsd.org/cgi/man.cgi?query=tar&sektion=5
//   http://www.gnu.org/software/tar/manual/html_node/Standard.html
//   http://pubs.opengroup.org/onlinepubs/9699919799/utilities/pax.html
package tar

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"time"
)

const (
	blockSize = 512

	// Types

	// 类型
	TypeReg           = '0'    // regular file // 普通文件
	TypeRegA          = '\x00' // regular file // 普通文件
	TypeLink          = '1'    // hard link // 硬链接
	TypeSymlink       = '2'    // symbolic link // 符号链接
	TypeChar          = '3'    // character device node // 字符设备节点
	TypeBlock         = '4'    // block device node // 块设备节点
	TypeDir           = '5'    // directory // 目录
	TypeFifo          = '6'    // fifo node // 先进先出队列节点
	TypeCont          = '7'    // reserved // 保留位
	TypeXHeader       = 'x'    // extended header // 扩展头
	TypeXGlobalHeader = 'g'    // global extended header // 全局扩展头
	TypeGNULongName   = 'L'    // Next file has a long name // 下一个文件记录有个长名字
	TypeGNULongLink   = 'K'    // Next file symlinks to a file w/ a long name // 下一个文件记录指向一个具有长名字的文件
	TypeGNUSparse     = 'S'    // sparse file // 稀疏文件
)

// A Header represents a single header in a tar archive.
// Some fields may not be populated.

// Header代表tar档案文件里的单个头。
// Header类型的某些字段可能未使用。
type Header struct {
	Name       string    // name of header file entry // 记录头域的文件名
	Mode       int64     // permission and mode bits // 权限和模式位
	Uid        int       // user id of owner // 所有者的用户ID
	Gid        int       // group id of owner // 所有者的组ID
	Size       int64     // length in bytes // 字节数（长度）
	ModTime    time.Time // modified time // 修改时间
	Typeflag   byte      // type of header entry // 记录头的类型
	Linkname   string    // target name of link // 链接的目标名
	Uname      string    // user name of owner // 所有者的用户名
	Gname      string    // group name of owner // 所有者的组名
	Devmajor   int64     // major number of character or block device // 字符设备或块设备的major number
	Devminor   int64     // minor number of character or block device // 字符设备或块设备的minor number
	AccessTime time.Time // access time // 访问时间
	ChangeTime time.Time // status change time // 状态改变时间
	Xattrs     map[string]string
}

// File name constants from the tar spec.
const (
	fileNameSize       = 100 // Maximum number of bytes in a standard tar name.
	fileNamePrefixSize = 155 // Maximum number of ustar extension bytes.
)

// FileInfo returns an os.FileInfo for the Header.

// FileInfo返回该Header对应的文件信息。（os.FileInfo类型）
func (h *Header) FileInfo() os.FileInfo {
	return headerFileInfo{h}
}

// headerFileInfo implements os.FileInfo.
type headerFileInfo struct {
	h *Header
}

func (fi headerFileInfo) Size() int64        { return fi.h.Size }
func (fi headerFileInfo) IsDir() bool        { return fi.Mode().IsDir() }
func (fi headerFileInfo) ModTime() time.Time { return fi.h.ModTime }
func (fi headerFileInfo) Sys() interface{}   { return fi.h }

// Name returns the base name of the file.
func (fi headerFileInfo) Name() string {
	if fi.IsDir() {
		return path.Base(path.Clean(fi.h.Name))
	}
	return path.Base(fi.h.Name)
}

// Mode returns the permission and mode bits for the headerFileInfo.
func (fi headerFileInfo) Mode() (mode os.FileMode) {
	// Set file permission bits.
	mode = os.FileMode(fi.h.Mode).Perm()

	// Set setuid, setgid and sticky bits.
	if fi.h.Mode&c_ISUID != 0 {
		// setuid
		mode |= os.ModeSetuid
	}
	if fi.h.Mode&c_ISGID != 0 {
		// setgid
		mode |= os.ModeSetgid
	}
	if fi.h.Mode&c_ISVTX != 0 {
		// sticky
		mode |= os.ModeSticky
	}

	// Set file mode bits.
	// clear perm, setuid, setgid and sticky bits.
	m := os.FileMode(fi.h.Mode) &^ 07777
	if m == c_ISDIR {
		// directory
		mode |= os.ModeDir
	}
	if m == c_ISFIFO {
		// named pipe (FIFO)
		mode |= os.ModeNamedPipe
	}
	if m == c_ISLNK {
		// symbolic link
		mode |= os.ModeSymlink
	}
	if m == c_ISBLK {
		// device file
		mode |= os.ModeDevice
	}
	if m == c_ISCHR {
		// Unix character device
		mode |= os.ModeDevice
		mode |= os.ModeCharDevice
	}
	if m == c_ISSOCK {
		// Unix domain socket
		mode |= os.ModeSocket
	}

	switch fi.h.Typeflag {
	case TypeLink, TypeSymlink:
		// hard link, symbolic link
		mode |= os.ModeSymlink
	case TypeChar:
		// character device node
		mode |= os.ModeDevice
		mode |= os.ModeCharDevice
	case TypeBlock:
		// block device node
		mode |= os.ModeDevice
	case TypeDir:
		// directory
		mode |= os.ModeDir
	case TypeFifo:
		// fifo node
		mode |= os.ModeNamedPipe
	}

	return mode
}

// sysStat, if non-nil, populates h from system-dependent fields of fi.
var sysStat func(fi os.FileInfo, h *Header) error

// Mode constants from the tar spec.
const (
	c_ISUID  = 04000   // Set uid
	c_ISGID  = 02000   // Set gid
	c_ISVTX  = 01000   // Save text (sticky bit)
	c_ISDIR  = 040000  // Directory
	c_ISFIFO = 010000  // FIFO
	c_ISREG  = 0100000 // Regular file
	c_ISLNK  = 0120000 // Symbolic link
	c_ISBLK  = 060000  // Block special file
	c_ISCHR  = 020000  // Character special file
	c_ISSOCK = 0140000 // Socket
)

// Keywords for the PAX Extended Header
const (
	paxAtime    = "atime"
	paxCharset  = "charset"
	paxComment  = "comment"
	paxCtime    = "ctime" // please note that ctime is not a valid pax header.
	paxGid      = "gid"
	paxGname    = "gname"
	paxLinkpath = "linkpath"
	paxMtime    = "mtime"
	paxPath     = "path"
	paxSize     = "size"
	paxUid      = "uid"
	paxUname    = "uname"
	paxXattr    = "SCHILY.xattr."
	paxNone     = ""
)

// FileInfoHeader creates a partially-populated Header from fi.
// If fi describes a symlink, FileInfoHeader records link as the link target.
// If fi describes a directory, a slash is appended to the name.
// Because os.FileInfo's Name method returns only the base name of
// the file it describes, it may be necessary to modify the Name field
// of the returned header to provide the full path name of the file.

// FileInfoHeader返回一个根据fi填写了部分字段的Header。
// 如果fi描述一个符号链接，FileInfoHeader函数将link参数作为链接目标。
// 如果fi描述一个目录，会在名字后面添加斜杠。
// 因为os.FileInfo接口的Name方法只返回它描述的文件的无路径名，
// 有可能需要将返回值的Name字段修改为文件的完整路径名。
func FileInfoHeader(fi os.FileInfo, link string) (*Header, error) {
	if fi == nil {
		return nil, errors.New("tar: FileInfo is nil")
	}
	fm := fi.Mode()
	h := &Header{
		Name:    fi.Name(),
		ModTime: fi.ModTime(),
		Mode:    int64(fm.Perm()), // or'd with c_IS* constants later
	}
	switch {
	case fm.IsRegular():
		h.Mode |= c_ISREG
		h.Typeflag = TypeReg
		h.Size = fi.Size()
	case fi.IsDir():
		h.Typeflag = TypeDir
		h.Mode |= c_ISDIR
		h.Name += "/"
	case fm&os.ModeSymlink != 0:
		h.Typeflag = TypeSymlink
		h.Mode |= c_ISLNK
		h.Linkname = link
	case fm&os.ModeDevice != 0:
		if fm&os.ModeCharDevice != 0 {
			h.Mode |= c_ISCHR
			h.Typeflag = TypeChar
		} else {
			h.Mode |= c_ISBLK
			h.Typeflag = TypeBlock
		}
	case fm&os.ModeNamedPipe != 0:
		h.Typeflag = TypeFifo
		h.Mode |= c_ISFIFO
	case fm&os.ModeSocket != 0:
		h.Mode |= c_ISSOCK
	default:
		return nil, fmt.Errorf("archive/tar: unknown file mode %v", fm)
	}
	if fm&os.ModeSetuid != 0 {
		h.Mode |= c_ISUID
	}
	if fm&os.ModeSetgid != 0 {
		h.Mode |= c_ISGID
	}
	if fm&os.ModeSticky != 0 {
		h.Mode |= c_ISVTX
	}
	if sysStat != nil {
		return h, sysStat(fi, h)
	}
	return h, nil
}

var zeroBlock = make([]byte, blockSize)

// POSIX specifies a sum of the unsigned byte values, but the Sun tar uses signed byte values.
// We compute and return both.
func checksum(header []byte) (unsigned int64, signed int64) {
	for i := 0; i < len(header); i++ {
		if i == 148 {
			// The chksum field (header[148:156]) is special: it should be treated as space bytes.
			unsigned += ' ' * 8
			signed += ' ' * 8
			i += 7
			continue
		}
		unsigned += int64(header[i])
		signed += int64(int8(header[i]))
	}
	return
}

type slicer []byte

func (sp *slicer) next(n int) (b []byte) {
	s := *sp
	b, *sp = s[0:n], s[n:]
	return
}

func isASCII(s string) bool {
	for _, c := range s {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

func toASCII(s string) string {
	if isASCII(s) {
		return s
	}
	var buf bytes.Buffer
	for _, c := range s {
		if c < 0x80 {
			buf.WriteByte(byte(c))
		}
	}
	return buf.String()
}
