# wvapp

`wvapp` is a lightweight library for creating cross-platform desktop applications using WebView. It leverages WebKit (Linux/macOS) and WebView2 (Windows) to render web content in native windows.

## Features

- **Cross-platform support**: Works on Windows, macOS, and Linux.
- **WebView integration**: Uses WebKit on Linux/macOS and WebView2 on Windows.
- **Customizable window options**: Configure window size, position, title, and more.
- **JavaScript bindings**: Bind JavaScript functions to native C/Go callbacks.
- **Debugging tools**: Enable developer tools for debugging web content.
- **Lightweight**: Minimal dependencies and optimized for performance.

## Compatibility and Stability

To maximize stability across different OS versions and graphics stacks, wvapp applies conservative defaults and offers opt-in overrides:

- Linux (GTK/WebKitGTK)
	- Defaults set only if not customized by user:
		- WEBKIT_DISABLE_DMABUF_RENDERER=1 (disable dmabuf to avoid black screen/crash on some drivers); override with WVAPP_DMABUF=1 to enable.
		- JSC_SIGNAL_FOR_GC=12 (SIGUSR2 as default GC signal); override with WVAPP_JSC_SIGNAL.
	- GTK3/GTK4 compatible code paths; WebKitGTK 4.1 preferred, auto-fallback to 4.0 in Makefile.
	- Event loop returns true when no active windows remain, ensuring Go Run() can exit.

- Windows (WebView2)
	- Fullscreen/maximize/minimize/restore unified APIs; DPI awareness enabled with layered fallback.
	- Event loop returns true on WM_QUIT to align exit semantics.

- macOS (WKWebView)
	- Bind/unbind uses a robust “clear-and-rebuild scripts” strategy to avoid stale handlers across navigations.
	- Event loop returns true when no windows remain, matching other platforms.

### Troubleshooting (Linux)
- If you see a black/blank window, try keeping dmabuf disabled (default). To test enabling it:
	- Set WVAPP_DMABUF=1 (which sets WEBKIT_DISABLE_DMABUF_RENDERER=0).
- Ensure WebKitGTK and GTK dev packages are installed; Makefile auto-detects webkit2gtk-4.1 and falls back to 4.0.
- On Wayland, verify XDG_RUNTIME_DIR is valid and has correct permissions.

### Stability Tips
- Prefer SetHtml for quick diagnostics (no network) before testing SetURL.
- Re-bind functions after navigation/DOMReady to ensure bridges are available.
- Avoid heavy work on the UI thread—offload to worker pool and use EvalJS for UI updates.
