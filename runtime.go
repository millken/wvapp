package wvapp

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"
	"unsafe"
)

//go:embed runtime.js
var runtimeJS []byte

func cString(s string) ([]byte, uintptr) {
	if s == "" {
		empty := []byte{0}
		return empty, uintptr(unsafe.Pointer(&empty[0]))
	}

	bytes := make([]byte, len(s)+1)
	copy(bytes, s)
	bytes[len(s)] = 0

	return bytes, uintptr(unsafe.Pointer(&bytes[0]))
}

func goString(c uintptr) string {
	if c == 0 {
		return ""
	}

	ptr := unsafe.Pointer(c)
	if ptr == nil {
		return ""
	}

	var length int
	for *(*byte)(unsafe.Add(ptr, uintptr(length))) != 0 {
		length++
		if length > 1000000 {
			break
		}
	}

	return string(unsafe.Slice((*byte)(ptr), length))
}

func init() {
	UserFunctionRegistry["_js_console_log"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		var logParts []string
		for _, arg := range args {
			logParts = append(logParts, fmt.Sprintf("%v", arg))
		}
		slog.Info("[JS Console]", "message", strings.Join(logParts, " "))
		return nil, nil
	}

	UserFunctionRegistry["_js_console_warn"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		var logParts []string
		for _, arg := range args {
			logParts = append(logParts, fmt.Sprintf("%v", arg))
		}
		slog.Warn("[JS Console]", "message", strings.Join(logParts, " "))
		return nil, nil
	}

	UserFunctionRegistry["_js_console_error"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		var logParts []string
		for _, arg := range args {
			logParts = append(logParts, fmt.Sprintf("%v", arg))
		}
		slog.Error("[JS Console]", "message", strings.Join(logParts, " "))
		return nil, nil
	}

	UserFunctionRegistry["_js_console_debug"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		var logParts []string
		for _, arg := range args {
			logParts = append(logParts, fmt.Sprintf("%v", arg))
		}
		slog.Debug("[JS Console]", "message", strings.Join(logParts, " "))
		return nil, nil
	}

	UserFunctionRegistry["_js_console_info"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		var logParts []string
		for _, arg := range args {
			logParts = append(logParts, fmt.Sprintf("%v", arg))
		}
		slog.Info("[JS Console]", "message", strings.Join(logParts, " "))
		return nil, nil
	}
	UserFunctionRegistry["_go_runtime_setTitle"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("missing title argument")
		}
		title, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid title argument")
		}
		if len(title) == 0 {
			return nil, fmt.Errorf("title cannot be empty")
		}
		wv.SetTitle(title)
		return nil, nil

	}
	UserFunctionRegistry["_go_runtime_setSize"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("missing width or height arguments")
		}
		width, ok := args[0].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid width argument")
		}
		height, ok := args[1].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid height argument")
		}
		if width < 100 || height < 100 {
			return nil, fmt.Errorf("width and height must be at least 100")
		}
		if width > 10000 || height > 10000 {
			return nil, fmt.Errorf("width and height must not exceed 10000")
		}
		wv.SetSize(int(width), int(height))
		return nil, nil
	}

	UserFunctionRegistry["_go_runtime_setFullscreen"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("missing fullscreen argument")
		}
		fullscreen, ok := args[0].(bool)
		if !ok {
			return nil, fmt.Errorf("invalid fullscreen argument")
		}
		wv.SetFullscreen(fullscreen)
		return nil, nil
	}

	UserFunctionRegistry["_go_runtime_maximizeWindow"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		wv.Maximize()
		return nil, nil
	}

	UserFunctionRegistry["_go_runtime_minimizeWindow"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		wv.Minimize()
		return nil, nil
	}

	UserFunctionRegistry["_go_runtime_restoreWindow"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		wv.Restore()
		return nil, nil
	}

	UserFunctionRegistry["_go_runtime_closeWindow"] = func(ctx context.Context, wv *Webview, args []any) (result any, err error) {
		wv.Terminate()
		return nil, nil
	}
}
