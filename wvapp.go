package wvapp

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type WindowPosition int32

const (
	WindowPositionCenter WindowPosition = iota
	WindowPositionLeftTop
	WindowPositionRightTop
	WindowPositionLeftBottom
	WindowPositionRightBottom
)

type WindowButtonFlag int32

// 窗口按钮位掩码
const (
	WindowButtonUnDefined WindowButtonFlag = 0
	WindowButtonMin       WindowButtonFlag = 1 << 0
	WindowButtonMax       WindowButtonFlag = 1 << 1
	WindowButtonClose     WindowButtonFlag = 1 << 2
	WindowButtonNone      WindowButtonFlag = 1 << 3 // 无按钮
	WindowButtonAll       WindowButtonFlag = WindowButtonMin | WindowButtonMax | WindowButtonClose
)

// 窗口选项结构体（仅用于初始化或批量设置）
type WindowOptions struct {
	Width            int
	Height           int
	MinWidth         int     // 最小宽度（0表示不限制）
	MinHeight        int     // 最小高度（0表示不限制）
	MaxWidth         int     // 最大宽度（0表示不限制）
	MaxHeight        int     // 最大高度（0表示不限制）
	ZoomLevel        float32 // 缩放级别（0表示默认缩放，1表示100%）
	ButtonFlags      WindowButtonFlag
	Position         WindowPosition // 窗口位置
	Debug            bool           // 是否开启开发者工具
	Title            string         // 窗口标题
	Icon             []byte         // 图标字节切片，通常是PNG或ICO格式
	Opaque           bool           // 窗口是否不透明（true=不透明，false=透明）
	HasShadow        bool           // 是否有阴影
	DisableResize    bool           // 是否禁用窗口大小调整（true=禁用，false=允许）
	EnableFileAccess bool           // 是否启用本地文件访问
	EnableClipboard  bool           // 是否启用剪贴板访问
	EnableWebGL      bool           // 是否启用WebGL
}

type cWebviewWindowOptions struct {
	width            int32
	height           int32
	minWidth         int32
	minHeight        int32
	maxWidth         int32
	maxHeight        int32
	buttonFlags      int32
	zoomLevel        float32
	position         int32
	_                [4]byte
	title            uintptr
	icon             uintptr
	iconLen          uint64
	opaque           bool
	hasShadow        bool
	debug            bool
	disableResize    bool
	enableFileAccess bool
	enableClipboard  bool
	enableWebGL      bool
	_                [1]byte
}

type EventType int

const (
	EventClose EventType = iota
	EventDomReady
)

type EventCallback func(wv *Webview, eventType EventType, userData unsafe.Pointer)

type BindCallback func(req string, userData unsafe.Pointer)

var (
	mainScheduler        = NewScheduler()
	windowCount          int32
	callbackRegistry     = make(map[*Webview]uintptr)
	callbackMutex        sync.Mutex
	bindCallbackRegistry = make(map[*Webview]map[string]uintptr)
	bindCallbackMutex    sync.Mutex
	runnerOnce           sync.Once
)

func Run() {
	runnerOnce.Do(func() {
		runtime.LockOSThread()
		mainScheduler.Start()

		for {
			mainScheduler.PollTasks()
			if done := webviewProcessEvents(); done {
				if atomic.LoadInt32(&windowCount) <= 0 {
					break
				}
			}
			time.Sleep(time.Millisecond * 5)
		}
	})
}

func PollMainTasks() {
	mainScheduler.PollTasks()
}

func ProcessEvents() bool {
	return webviewProcessEvents()
}
