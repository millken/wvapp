//go:build darwin || linux
// +build darwin linux

package wvapp

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/ebitengine/purego"
)

func libraryPath() string {
	var name string
	var paths []string

	wvappPath := os.Getenv("WVAPP_PATH")
	execPath, _ := os.Executable()
	dir := filepath.Dir(execPath)

	switch runtime.GOOS {
	case "linux":
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		os.Setenv("JSC_SIGNAL_FOR_GC", "12")
		name = "libwvapp.so"
		paths = []string{wvappPath, dir}
	case "darwin":
		name = "libwvapp.dylib"
		paths = []string{wvappPath, dir, filepath.Join(dir, "..", "Frameworks")}
	}

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
	return purego.Dlopen(name, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
}

func loadSymbol(lib uintptr, name string) uintptr {
	ptr, err := purego.Dlsym(lib, name)
	if err != nil {
		panic("rgfw: failed to load symbol " + name + ": " + err.Error())
	}
	return ptr
}
