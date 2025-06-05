package wvapp

import (
	"os"
	"runtime"
	"syscall"
	"testing"
)

func TestLoad(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("This test is only for Windows")
	}

	libPath := libraryPath()
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Fatalf("Library not found: %s", libPath)
	}

	lib, err := loadLibrary(libPath)
	if err != nil {
		t.Fatalf("Failed to load library: %v", err)
	}
	defer syscall.FreeLibrary(syscall.Handle(lib))

	symbols := []string{
		"wvapp_create",
		"wvapp_set_html",
		"wvapp_bind",
	}

	for _, sym := range symbols {
		if ptr := loadSymbol(lib, sym); ptr == 0 {
			t.Fatalf("Failed to load symbol: %s", sym)
		}
	}
	t.Logf("Successfully loaded library and symbols from %s", libPath)
}
