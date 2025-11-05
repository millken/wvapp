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
		// Linux: 仅在未由用户显式设置时配置兼容性环境变量
		// WEBKIT_DISABLE_DMABUF_RENDERER
		// 默认禁用 dmabuf 渲染以规避部分发行版/驱动上的黑屏或崩溃问题
		// 可通过设置 WVAPP_DMABUF=1 显式启用（将该变量置 0）
		if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
			defaultDmabuf := "1" // 1=禁用 dmabuf（更保守，兼容性优先）
			if v := os.Getenv("WVAPP_DMABUF"); v == "1" || v == "true" || v == "enable" {
				defaultDmabuf = "0"
			}
			os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", defaultDmabuf)
		}

		// JSC_SIGNAL_FOR_GC
		// 仅在未设置时提供一个保守的默认值，可通过 WVAPP_JSC_SIGNAL 覆盖
		if os.Getenv("JSC_SIGNAL_FOR_GC") == "" {
			sig := os.Getenv("WVAPP_JSC_SIGNAL")
			if sig == "" {
				sig = "12" // 默认使用 12（SIGUSR2）
			}
			os.Setenv("JSC_SIGNAL_FOR_GC", sig)
		}
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
