package filetree

import (
	"archive/tar"
	"io"
	"os"
	"strconv"

	"github.com/cespare/xxhash"
	"github.com/sirupsen/logrus"
)

// FileInfo contains tar metadata for a specific FileNode
type FileInfo struct {
	Path     string      `json:"path"`
	TypeFlag byte        `json:"typeFlag"`
	Linkname string      `json:"linkName"`
	Hash     uint64      `json:"hash"`
	Size     int64       `json:"size"`
	Mode     os.FileMode `json:"fileMode"`
	Uid      int         `json:"uid"`
	Gid      int         `json:"gid"`
	IsDir    bool        `json:"isDir"`
}

// NewFileInfoFromTarHeader extracts the metadata from a tar header and file contents and generates a new FileInfo object.
func NewFileInfoFromTarHeader(reader *tar.Reader, header *tar.Header, path string) FileInfo {
	var hash uint64
	if header.Typeflag != tar.TypeDir {
		// TODO: Pass path to getHashFromReader
		// Create a new FileNode having path. Call Path() of FileNode
		// Compute hash from contents and absolute path
		hash = getHashFromReader(reader, path)
	}

	return FileInfo{
		Path:     path,
		TypeFlag: header.Typeflag,
		Linkname: header.Linkname,
		Hash:     hash,
		Size:     header.FileInfo().Size(),
		Mode:     header.FileInfo().Mode(),
		Uid:      header.Uid,
		Gid:      header.Gid,
		IsDir:    header.FileInfo().IsDir(),
	}
}

func NewFileInfo(realPath, path string, info os.FileInfo) FileInfo {
	var err error

	// todo: don't use tar types here, create our own...
	var fileType byte
	var linkName string
	var size int64

	if info.Mode()&os.ModeSymlink != 0 {
		fileType = tar.TypeSymlink

		linkName, err = os.Readlink(realPath)
		if err != nil {
			logrus.Panic("unable to read link:", realPath, err)
		}

	} else if info.IsDir() {
		fileType = tar.TypeDir
	} else {
		fileType = tar.TypeReg

		size = info.Size()
	}

	var hash uint64
	if fileType != tar.TypeDir {
		file, err := os.Open(realPath)
		if err != nil {
			logrus.Panic("unable to read file:", realPath)
		}
		defer file.Close()
		hash = getHashFromReader(file, path)
	}

	return FileInfo{
		Path:     path,
		TypeFlag: fileType,
		Linkname: linkName,
		Hash:     hash,
		Size:     size,
		Mode:     info.Mode(),
		// todo: support UID/GID
		Uid:   -1,
		Gid:   -1,
		IsDir: info.IsDir(),
	}
}

// Copy duplicates a FileInfo
func (data *FileInfo) Copy() *FileInfo {
	if data == nil {
		return nil
	}
	return &FileInfo{
		Path:     data.Path,
		TypeFlag: data.TypeFlag,
		Linkname: data.Linkname,
		Hash:     data.Hash,
		Size:     data.Size,
		Mode:     data.Mode,
		Uid:      data.Uid,
		Gid:      data.Gid,
		IsDir:    data.IsDir,
	}
}

// Compare determines the DiffType between two FileInfos based on the type and contents of each given FileInfo
func (data *FileInfo) Compare(other FileInfo) DiffType {
	if data.TypeFlag == other.TypeFlag {
		if data.Hash == other.Hash &&
			data.Mode == other.Mode &&
			data.Uid == other.Uid &&
			data.Gid == other.Gid {
			return Unmodified
		}
	}
	return Modified
}

func getHashFromReader(reader io.Reader, path string) uint64 {
	h := xxhash.New()

	// Get absolute path
	tmpFileNode := FileNode{
		path: path,
	}
	absPath := tmpFileNode.Path()

	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			logrus.Panic(err)
		}
		if n == 0 {
			break
		}

		_, err = h.Write(buf[:n])
		if err != nil {
			logrus.Panic(err)
		}
	}

	contentHash := h.Sum64()
	contentHashStr := strconv.FormatUint(contentHash, 10)
	finalHash := xxhash.New()

	_, err := finalHash.Write([]byte(contentHashStr))
	if err != nil {
		logrus.Panic(err)
	}
	// Add absolute path as hash parameter
	_, err = finalHash.Write([]byte(absPath))
	if err != nil {
		logrus.Panic(err)
	}

	return finalHash.Sum64()
}
