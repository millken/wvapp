package wvapp

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego"
)

// ResourceHandler 资源处理函数类型
type ResourceHandler func(path string) *Resource

// Resource 表示URI的资源
type Resource struct {
	Content     []byte
	ContentType string
	IsEmbed     bool // 是否为静态资源,用于静态资源零拷贝
}

type cWebviewResource struct {
	content       uintptr
	contentLength uint64
	mimeType      uintptr
}

var (
	globalResourceHandler atomic.Value

	uriSchemeLoadOnce sync.Once
	uriSchemeInitErr  error

	// URI相关函数
	webviewRegisterGlobalURIScheme func(uintptr, uintptr) int32
	webviewGetGlobalURIScheme      func() uintptr
	webviewCleanupGlobalURIScheme  func()
	webviewCreateResource          func(uintptr, uint64, uintptr, uintptr) uintptr
)

func cResourceHandler(pathPtr uintptr) uintptr {
	if pathPtr == 0 {
		return 0
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in resource handler", "error", r, "path", goString(pathPtr))
		}
	}()

	path := goString(pathPtr)

	if h := globalResourceHandler.Load(); h != nil {
		if handler, ok := h.(ResourceHandler); ok && handler != nil {
			resource := handler(path)
			if resource == nil {
				slog.Warn("Resource not found", "path", path)
				return 0
			}

			contentBytes, contentPtr := cString(string(resource.Content))
			mimeBytes, mimePtr := cString(resource.ContentType)

			if webviewCreateResource == nil {
				slog.Error("webviewCreateResource function not available")
				return 0
			}

			var isEmbedArg uintptr
			if resource.IsEmbed {
				isEmbedArg = 1
			} else {
				isEmbedArg = 0
			}
			resourcePtr := webviewCreateResource(contentPtr, uint64(len(resource.Content)), mimePtr, isEmbedArg)

			runtime.KeepAlive(contentBytes)
			runtime.KeepAlive(mimeBytes)

			if resourcePtr == 0 {
				slog.Error("Failed to create C resource", "path", path)
				return 0
			}

			return resourcePtr
		}
	}

	slog.Warn("No resource handler registered", "path", path)
	return 0
}

func ensureLibraryLoaded() error {
	uriSchemeLoadOnce.Do(func() {
		lib := libraryPath()
		handle, err := loadLibrary(lib)
		if err != nil {
			uriSchemeInitErr = fmt.Errorf("uri_scheme: failed to load library %s: %w", lib, err)
			return
		}

		purego.RegisterLibFunc(&webviewRegisterGlobalURIScheme, handle, "webview_register_global_uri_scheme")
		purego.RegisterLibFunc(&webviewGetGlobalURIScheme, handle, "webview_get_global_uri_scheme")
		purego.RegisterLibFunc(&webviewCleanupGlobalURIScheme, handle, "webview_cleanup_global_uri_scheme")
		purego.RegisterLibFunc(&webviewCreateResource, handle, "webview_create_resource")

		slog.Debug("URI scheme functions loaded successfully")
	})

	if uriSchemeInitErr != nil {
		return uriSchemeInitErr
	}

	if webviewRegisterGlobalURIScheme == nil {
		return fmt.Errorf("webview_register_global_uri_scheme function not available")
	}
	return nil
}

// NewResourceHandlerFromFS 从文件系统创建资源处理函数
func NewResourceHandlerFromFS(fsys fs.FS) ResourceHandler {
	if fsys == nil {
		return func(path string) *Resource {
			slog.Error("File system is nil")
			return nil
		}
	}
	return func(path string) *Resource {
		path = normalizePath(filepath.Clean(path))
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic in resource handler from FS", "error", r, "path", path)
			}
		}()
		f, err := fsys.Open(path)
		if err != nil {
			slog.Warn("Resource not found in FS", "path", path, "error", err)
			return nil
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil || len(data) == 0 {
			return nil
		}
		mimeType := mime.TypeByExtension(filepath.Ext(path))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		return &Resource{
			Content:     data,
			ContentType: mimeType,
			IsEmbed:     false,
		}
	}
}
func normalizePath(path string) string {
	const prefix = "index.html/"
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	if strings.HasPrefix(path, prefix) {
		path = path[11:]
	}
	if strings.HasSuffix(path, "/") || path == "" {
		path += "index.html"
	}
	return path
}

// NewResourceHandlerFromStaticCache 从静态缓存创建资源处理函数
func NewResourceHandlerFromStaticCache(staticCache map[string][]byte) ResourceHandler {
	return func(path string) *Resource {
		path = normalizePath(filepath.Clean(path))

		data, ok := staticCache[path]
		if !ok {
			return nil
		}
		mimeType := mime.TypeByExtension(filepath.Ext(path))
		if mimeType == "" {
			//TODO: 从body中推断MIME类型 github.com/gabriel-vasile/mimetype
			mimeType = "application/octet-stream"
		}
		return &Resource{
			Content:     data,
			ContentType: mimeType,
			IsEmbed:     true,
		}
	}
}

// RegisterGlobalURIScheme 注册全局URI
func RegisterGlobalURIScheme(schemeName string, handler ResourceHandler) error {
	if err := ensureLibraryLoaded(); err != nil {
		return err
	}

	if schemeName == "" {
		return fmt.Errorf("scheme name cannot be empty")
	}

	if handler == nil {
		return fmt.Errorf("resource handler cannot be nil")
	}

	globalResourceHandler.Store(handler)

	cHandler := purego.NewCallback(cResourceHandler)

	schemeBytes, schemePtr := cString(schemeName)

	var result int32
	if mainScheduler != nil {
		result = mainScheduler.RunInMainThreadWithResult(func() any {
			ret := webviewRegisterGlobalURIScheme(schemePtr, cHandler)
			return int32(ret)
		}).(int32)
	} else {
		// 直接调用（适用于主线程环境）
		result = webviewRegisterGlobalURIScheme(schemePtr, cHandler)
	}

	runtime.KeepAlive(schemeBytes)
	runtime.KeepAlive(cHandler)

	if result != 0 {
		var nilHandler ResourceHandler
		globalResourceHandler.Store(nilHandler)
		return fmt.Errorf("failed to register URI scheme '%s': error code %d", schemeName, result)
	}

	return nil
}

// GetGlobalURIScheme 获取当前注册的URI名
func GetGlobalURIScheme() string {
	if err := ensureLibraryLoaded(); err != nil {
		return ""
	}

	if webviewGetGlobalURIScheme == nil {
		return ""
	}

	var result uintptr
	if mainScheduler != nil {
		result = mainScheduler.RunInMainThreadWithResult(func() any {
			return webviewGetGlobalURIScheme()
		}).(uintptr)
	} else {
		result = webviewGetGlobalURIScheme()
	}

	if result == 0 {
		return ""
	}

	return goString(result)
}

// CleanupGlobalURIScheme 清理全局URI
func CleanupGlobalURIScheme() error {
	if err := ensureLibraryLoaded(); err != nil {
		return fmt.Errorf("failed to ensure library loaded: %w", err)
	}

	if webviewCleanupGlobalURIScheme == nil {
		return fmt.Errorf("webview_cleanup_global_uri_scheme function not available")
	}

	var nilHandler ResourceHandler
	globalResourceHandler.Store(nilHandler)

	if mainScheduler != nil {
		mainScheduler.RunInMainThread(func() {
			webviewCleanupGlobalURIScheme()
		})
	} else {
		webviewCleanupGlobalURIScheme()
	}

	return nil
}

// RegisterGlobalURISchemeWithFS 使用文件系统注册全局URI
func RegisterGlobalURISchemeWithFS(schemeName string, fsys fs.FS) error {
	return RegisterGlobalURIScheme(schemeName, NewResourceHandlerFromFS(fsys))
}
