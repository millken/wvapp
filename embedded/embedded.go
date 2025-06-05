package embedded

import (
	_ "embed"
	"hash/crc32"
	"os"
	"path/filepath"
)

func fileCRC32(path string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	h := crc32.NewIEEE()
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return h.Sum32(), nil
}

func Init() error {
	if os.Getenv("WVAPP_PATH") != "" {
		return nil
	}

	dir := filepath.Join(os.TempDir(), "wvapp")
	file := filepath.Join(dir, name)
	if fi, err := os.Stat(file); err == nil {
		needUpdate := false
		if fi.Size() != int64(len(lib)) {
			needUpdate = true
		} else {
			fileCRC, err1 := fileCRC32(file)
			libCRC := crc32.ChecksumIEEE(lib)
			if err1 != nil || fileCRC != libCRC {
				needUpdate = true
			}
		}
		if needUpdate {
			if err := os.Remove(file); err != nil {
				return err
			}
			if err := os.WriteFile(file, lib, os.ModePerm); err != nil { //nolint:gosec
				return err
			}
		}
	} else {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil { //nolint:gosec
			return err
		}
		if err := os.WriteFile(file, lib, os.ModePerm); err != nil { //nolint:gosec
			return err
		}
	}

	if err := os.Setenv("WVAPP_PATH", dir); err != nil {
		return err
	}
	return nil
}
