package wvapp

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

type Webview struct{}

var (
	loadOnce                 sync.Once
	webviewCreate            func(*cWebviewWindowOptions) *Webview
	webviewSetUrl            func(*Webview, uintptr)
	webviewSetHtml           func(*Webview, uintptr)
	webviewSetTitle          func(*Webview, uintptr)
	webviewSetSize           func(*Webview, int, int)
	webviewSetWindowPosition func(*Webview, WindowPosition)
	webviewSetDebug          func(*Webview, bool)
	webviewSetFullscreen     func(*Webview, bool)
	webviewSetWindowButtons  func(*Webview, WindowButtonFlag)
	webviewEvalJS            func(*Webview, uintptr)
	webviewTerminate         func(*Webview)
	webviewSetEventCallback  func(*Webview, uintptr, unsafe.Pointer)
	webviewBind              func(*Webview, uintptr, uintptr, unsafe.Pointer)
	webviewUnbind            func(*Webview, uintptr)
	webviewProcessEvents     func() bool // 返回true表示应该退出循环
	webviewMaximize          func(*Webview)
	webviewMinimize          func(*Webview)
	webviewRestore           func(*Webview)
)

func NewWebview(options *WindowOptions) (*Webview, error) {
	if options == nil {
		options = &WindowOptions{
			Width:         800,
			Height:        600,
			MinWidth:      400,
			MinHeight:     300,
			MaxWidth:      1600,
			MaxHeight:     1200,
			ZoomLevel:     1.0,
			ButtonFlags:   WindowButtonAll,
			Position:      WindowPositionCenter,
			Debug:         false,
			Title:         "Webview",
			Icon:          nil,
			Opaque:        true,
			HasShadow:     true,
			DisableResize: false,
		}
	}
	var initErr error
	loadOnce.Do(func() {
		lib := libraryPath()
		handle, err := loadLibrary(lib)
		if err != nil {
			initErr = fmt.Errorf("webview: failed to load library %s: %w", lib, err)
			return
		}
		purego.RegisterLibFunc(&webviewCreate, handle, "webview_create")
		purego.RegisterLibFunc(&webviewSetUrl, handle, "webview_set_url")
		purego.RegisterLibFunc(&webviewSetHtml, handle, "webview_set_html")
		purego.RegisterLibFunc(&webviewSetTitle, handle, "webview_set_title")
		purego.RegisterLibFunc(&webviewSetSize, handle, "webview_set_size")
		purego.RegisterLibFunc(&webviewSetWindowPosition, handle, "webview_set_window_position")
		purego.RegisterLibFunc(&webviewSetDebug, handle, "webview_set_debug")
		purego.RegisterLibFunc(&webviewSetFullscreen, handle, "webview_set_fullscreen")
		purego.RegisterLibFunc(&webviewSetWindowButtons, handle, "webview_set_window_buttons")
		purego.RegisterLibFunc(&webviewEvalJS, handle, "webview_eval_js")
		purego.RegisterLibFunc(&webviewProcessEvents, handle, "webview_process_events")
		purego.RegisterLibFunc(&webviewTerminate, handle, "webview_terminate")
		purego.RegisterLibFunc(&webviewSetEventCallback, handle, "webview_set_event_callback")
		purego.RegisterLibFunc(&webviewBind, handle, "webview_bind")
		purego.RegisterLibFunc(&webviewUnbind, handle, "webview_unbind")
		purego.RegisterLibFunc(&webviewMaximize, handle, "webview_maximize")
		purego.RegisterLibFunc(&webviewMinimize, handle, "webview_minimize")
		purego.RegisterLibFunc(&webviewRestore, handle, "webview_restore")
	})
	if initErr != nil {
		return nil, initErr
	}
	atomic.AddInt32(&windowCount, 1)

	var titlePtr, iconPtr uintptr
	var iconLen uint64
	var titleBytes, iconBytes []byte
	if options.Title != "" {
		titleBytes = append([]byte(options.Title), 0)
		titlePtr = uintptr(unsafe.Pointer(&titleBytes[0]))
	}
	if len(options.Icon) > 0 {
		iconBytes = options.Icon
		iconPtr = uintptr(unsafe.Pointer(&iconBytes[0]))
		iconLen = uint64(len(iconBytes))
	}

	cOptions := &cWebviewWindowOptions{
		width:         int32(options.Width),
		height:        int32(options.Height),
		minWidth:      int32(options.MinWidth),
		minHeight:     int32(options.MinHeight),
		maxWidth:      int32(options.MaxWidth),
		maxHeight:     int32(options.MaxHeight),
		zoomLevel:     options.ZoomLevel,
		buttonFlags:   int32(options.ButtonFlags),
		position:      int32(options.Position),
		debug:         options.Debug,
		title:         titlePtr,
		icon:          iconPtr,
		iconLen:       iconLen,
		disableResize: options.DisableResize,
		opaque:        options.Opaque,
		hasShadow:     options.HasShadow,
	}

	wv := mainScheduler.RunInMainThreadWithResult(func() any {
		return webviewCreate(cOptions)
	}).(*Webview)

	runtime.KeepAlive(titleBytes)
	runtime.KeepAlive(iconBytes)

	if wv == nil {
		atomic.AddInt32(&windowCount, -1)
		return nil, fmt.Errorf("webview: failed to create webview instance")
	}

	wv.SetEventCallback(nil)
	return wv, nil
}

func (w *Webview) SetURL(url string) {
	if url == "" {
		return // 避免传递空URL
	}

	mainScheduler.RunInMainThread(func() {
		if webviewSetUrl == nil || w == nil {
			return
		}

		cstr, ptr := cString(url)
		webviewSetUrl(w, ptr)
		// 保持字符串在内存中直到函数返回
		runtime.KeepAlive(cstr)
	})
}

func (w *Webview) SetTitle(title string) {
	mainScheduler.RunInMainThread(func() {
		cstr, ptr := cString(title)
		webviewSetTitle(w, ptr)
		runtime.KeepAlive(cstr)
	})
}

func (w *Webview) SetSize(width, height int) {
	mainScheduler.RunInMainThread(func() { webviewSetSize(w, width, height) })
}

func (w *Webview) SetHtml(html string) {
	mainScheduler.RunInMainThread(func() {
		cstr, ptr := cString(html)
		webviewSetHtml(w, ptr)
		runtime.KeepAlive(cstr)
	})
}

func (w *Webview) SetWindowPosition(position WindowPosition) {
	mainScheduler.RunInMainThread(func() { webviewSetWindowPosition(w, position) })
}

func (w *Webview) SetDebug(debug bool) {
	mainScheduler.RunInMainThread(func() { webviewSetDebug(w, debug) })
}

func (w *Webview) SetFullscreen(fullscreen bool) {
	mainScheduler.RunInMainThread(func() { webviewSetFullscreen(w, fullscreen) })
}

func (w *Webview) SetWindowButtons(buttons WindowButtonFlag) {
	mainScheduler.RunInMainThread(func() { webviewSetWindowButtons(w, buttons) })
}

func (w *Webview) EvalJS(js string) {
	mainScheduler.RunInMainThread(func() {
		cstr, ptr := cString(js)
		webviewEvalJS(w, ptr)
		runtime.KeepAlive(cstr)
	})
}

func (w *Webview) SetEventCallback(callback EventCallback) {
	callbackWrapper := func(wv *Webview, eventType int32, userData unsafe.Pointer) uintptr {
		if callback != nil {
			callback(wv, EventType(eventType), userData)
		}
		switch EventType(eventType) {
		case EventDomReady:
			if len(runtimeJS) > 0 {
				wv.EvalJS(unsafe.String(&runtimeJS[0], len(runtimeJS)))
			}

		case EventClose:
			// 窗口关闭时清理 Go 端资源
			atomic.AddInt32(&windowCount, -1)
			callbackMutex.Lock()
			delete(callbackRegistry, wv)
			callbackMutex.Unlock()

			bindCallbackMutex.Lock()
			delete(bindCallbackRegistry, wv)
			bindCallbackMutex.Unlock()
		}
		return 0 // 返回 uintptr 类型的值
	}

	fn := purego.NewCallback(callbackWrapper)
	callbackMutex.Lock()
	callbackRegistry[w] = fn
	callbackMutex.Unlock()
	mainScheduler.RunInMainThread(func() {
		webviewSetEventCallback(w, fn, nil)
	})
}

func (w *Webview) Bind(name string, fn BindCallback, userData unsafe.Pointer) {
	if name == "" || fn == nil || w == nil {
		return
	}

	cstrName, namePtr := cString(name)

	goCallback := func(req uintptr, cbUserData unsafe.Pointer) uintptr {
		reqStr := goString(req)
		if fn != nil && reqStr != "" {
			fn(reqStr, cbUserData)
		}
		return 0 // 返回 uintptr 类型的值
	}

	cCallbackPtr := purego.NewCallback(goCallback)

	bindCallbackMutex.Lock()
	if _, ok := bindCallbackRegistry[w]; !ok {
		bindCallbackRegistry[w] = make(map[string]uintptr)
	}
	bindCallbackRegistry[w][name] = cCallbackPtr
	bindCallbackMutex.Unlock()

	mainScheduler.RunInMainThread(func() {
		if webviewBind != nil {
			webviewBind(w, namePtr, cCallbackPtr, userData)
		}
	})

	runtime.KeepAlive(cstrName)
}

func (w *Webview) Unbind(name string) {
	cstrName, namePtr := cString(name)
	mainScheduler.RunInMainThread(func() {
		webviewUnbind(w, namePtr)
	})
	bindCallbackMutex.Lock()
	if webviewBinds, ok := bindCallbackRegistry[w]; ok {
		delete(webviewBinds, name)
		if len(webviewBinds) == 0 {
			delete(bindCallbackRegistry, w)
		}
	}
	bindCallbackMutex.Unlock()
	runtime.KeepAlive(cstrName)
}

func (w *Webview) Destroy() {
	mainScheduler.RunInMainThread(func() { webviewTerminate(w) })
}

func (w *Webview) Maximize() {
	mainScheduler.RunInMainThread(func() {
		if webviewMaximize != nil && w != nil {
			webviewMaximize(w)
		}
	})
}

func (w *Webview) Minimize() {
	mainScheduler.RunInMainThread(func() {
		if webviewMinimize != nil && w != nil {
			webviewMinimize(w)
		}
	})
}

func (w *Webview) Restore() {
	mainScheduler.RunInMainThread(func() {
		if webviewRestore != nil && w != nil {
			webviewRestore(w)
		}
	})
}

func (w *Webview) Terminate() {
	mainScheduler.RunInMainThread(func() {
		if webviewTerminate != nil && w != nil {
			webviewTerminate(w)
		}
	})
}

func (w *Webview) InitializeJavaScriptRuntime() {
	InitializeGlobalWorkerPool(4, 100) // Default: 4 workers, queue size 100

	w.Bind("_runtime_invoke", func(req string, userData unsafe.Pointer) {
		if req == "" {
			fmt.Fprintln(os.Stderr, "Runtime Error: Received empty request from JS.")
			return
		}
		var p CallPayload
		if err := json.Unmarshal([]byte(req), &p); err != nil {
			fmt.Fprintf(os.Stderr, "Runtime Error: Failed to unmarshal request from JS: %v. Request: %s\n", err, req)
			return
		}

		handler, ok := UserFunctionRegistry[p.Func]
		if !ok {
			fmt.Fprintf(os.Stderr, "Runtime Error: Function '%s' not found in UserFunctionRegistry.\n", p.Func)
			if p.PromiseID != 0 { // If JS expects a response
				errorMsgJSON, _ := json.Marshal(fmt.Sprintf("Function '%s' not found", p.Func))
				rejectScript := fmt.Sprintf("window._rejectWebviewPromise(%d, %s);", p.PromiseID, string(errorMsgJSON))
				w.EvalJS(rejectScript)
			}
			return
		}

		job := Job{
			Webview: w,
			Payload: p,
			Handler: handler,
		}

		if err := globalWorkerPool.Submit(job); err != nil {
			fmt.Fprintf(os.Stderr, "Runtime Error: Failed to submit job for '%s' to worker pool: %v\n", p.Func, err)
			if p.PromiseID != 0 { // If JS expects a response
				errorMsgJSON, _ := json.Marshal(fmt.Sprintf("Failed to queue task for '%s': %s", p.Func, err.Error()))
				rejectScript := fmt.Sprintf("window._rejectWebviewPromise(%d, %s);", p.PromiseID, string(errorMsgJSON))
				w.EvalJS(rejectScript)
			}
		}
		// The _runtime_invoke callback returns quickly, job is now in the worker pool.
	}, nil)
}
