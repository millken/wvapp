package wvapp

import (
	"os"
	"path/filepath"
	"syscall"
)

func libraryPath() string {
	var name string = "wvapp.dll"
	var paths []string

	wvappPath := os.Getenv("WVAPP_PATH")
	execPath, _ := os.Executable()
	dir := filepath.Dir(execPath)
	paths = []string{wvappPath, dir}
	for _, v := range paths {
		n := filepath.Join(v, name)
		if _, err := os.Stat(n); err == nil {
			name = n
			break
		}
	}

	return name
}

func loadLibrary(name string) (uintptr, error) {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return 0, err
	}
	name = filepath.Clean(name)
	handle, err := syscall.LoadLibrary(name)
	return uintptr(handle), err
}

func loadSymbol(lib uintptr, name string) uintptr {
	ptr, err := syscall.GetProcAddress(syscall.Handle(lib), name)
	if err != nil {
		panic("wvapp: failed to load symbol " + name + ": " + err.Error())
	}
	return ptr
}
