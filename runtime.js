window.runtime_env = "webview";
// --- Promise Management Start ---
// 用于存储待处理的 Promise 的 resolve/reject 函数
window._webviewPromises = {};
// 用于生成唯一的 Promise ID
window._webviewPromiseNextId = 1;
// 保存原始 console 方法，避免循环调用
window._originalConsole = {};

// Go 端调用此函数来 resolve 一个 Promise
window._resolveWebviewPromise = function(id, value) {
    if (window._originalConsole.debug) {
        window._originalConsole.debug("_resolveWebviewPromise called with ID:", id, "Value:", value);
    }
    if (window._webviewPromises[id]) {
        if (window._webviewPromises[id].timeout) {
            clearTimeout(window._webviewPromises[id].timeout);
        }
        window._webviewPromises[id].resolve(value);
        delete window._webviewPromises[id];
    } else {
        if (window._originalConsole.warn) {
            window._originalConsole.warn("Could not resolve webview promise: ID " + id + " not found. Value:", value);
        }
    }
};

// Go 端调用此函数来 reject 一个 Promise
window._rejectWebviewPromise = function(id, error) {
    if (window._originalConsole.log) {
        window._originalConsole.log("_rejectWebviewPromise called with ID:", id);
    }
    if (window._webviewPromises[id]) {
        if (window._webviewPromises[id].timeout) {
            clearTimeout(window._webviewPromises[id].timeout);
        }
        const errorObj = (typeof error === 'string' || typeof error === 'object' && !(error instanceof Error)) ? 
                         new Error(typeof error === 'object' ? JSON.stringify(error) : error) : 
                         error;
        window._webviewPromises[id].reject(errorObj);
        delete window._webviewPromises[id];
    } else {
        if (window._originalConsole.warn) {
            window._originalConsole.warn("Could not reject webview promise: ID " + id + " not found. Error:", error);
        }
    }
};
// --- Promise Management End ---

// --- Go Call Helper Function ---
/**
 * 调用一个已绑定的 Go 函数。
 * @param {string} goFuncName - 要调用的 Go 函数的绑定名称 (例如 "_go_runtime_setTitle")。
 * @param {Array<any>} funcArgs - 调用 Go 函数时传递的参数数组。
 * @param {boolean} [expectResponse=true] - 是否期望从 Go 函数获得响应 (通过 Promise)。
 * @returns {Promise<any> | void} - 如果 expectResponse 为 true，则返回一个 Promise；否则返回 void。
 */
function goCall(goFuncName, funcArgs = [], expectResponse = false) {
    if (typeof window._runtime_invoke !== 'function') {
        const errorMessage = `Webview native bridge (window._runtime_invoke) is not available. Cannot call Go function: ${goFuncName}`;
        if (window._originalConsole && window._originalConsole.error) {
            window._originalConsole.error(errorMessage);
        }
        if (expectResponse) {
            return Promise.reject(new Error(errorMessage));
        }
        return;
    }

    const payload = {
        func: goFuncName, // Go 函数的绑定名称
        args: funcArgs    // 传递给 Go 函数的参数
    };

    if (expectResponse) {
        return new Promise((resolve, reject) => {
            const promiseId = window._webviewPromiseNextId++;
            
            // 添加超时机制防止内存泄漏
            const timeout = setTimeout(() => {
                if (window._webviewPromises[promiseId]) {
                    delete window._webviewPromises[promiseId];
                    reject(new Error(`Timeout waiting for response from ${goFuncName} (30s)`));
                }
            }, 30000); // 30秒超时
            
            window._webviewPromises[promiseId] = { resolve, reject, timeout };
            payload.promiseId = promiseId; // 只有需要响应时才包含 promiseId

            try {
                window._runtime_invoke(JSON.stringify(payload));
            } catch (e) {
                clearTimeout(timeout);
                delete window._webviewPromises[promiseId]; // 清理
                if (window._originalConsole && window._originalConsole.error) {
                    window._originalConsole.error(`Error invoking native bridge for ${goFuncName} (expecting response):`, e);
                }
                reject(e);
            }
        });
    } else {
        // 对于不需要响应的调用，消息体是 { func: "...", args: [...] }
        try {
            window._runtime_invoke(JSON.stringify(payload));
        } catch (e) {
            if (window._originalConsole && window._originalConsole.error) {
                window._originalConsole.error(`Error invoking native bridge for ${goFuncName} (no response expected):`, e);
            }
        }
        return;
    }
}
// --- Go Call Helper Function End ---

window.runtime = {
    SetTitle: function(title) {
        return goCall('_go_runtime_setTitle', [title]);
    },
    SetSize: function(width, height) {
        return goCall('_go_runtime_setSize', [width, height]);
    },
    SetFullscreen: function(fullscreen) {
        return goCall('_go_runtime_setFullscreen', [fullscreen]);
    },
    SetFrameless: function(frameless) {
        return goCall('_go_runtime_setFrameless', [frameless]);
    },
    BeginDragAt: function(x, y) {
        return goCall('_go_runtime_beginDragAt', [x, y]);
    },
    MinimizeWindow: function() {
        return goCall('_go_runtime_minimizeWindow', []);
    },
    MaximizeWindow: function() {
        return goCall('_go_runtime_maximizeWindow', []);
    },
    RestoreWindow: function() {
        return goCall('_go_runtime_restoreWindow', []);
    },
    CloseWindow: function() {
        return goCall('_go_runtime_closeWindow', []);
    }
};
// From: https://stackoverflow.com/questions/105034/how-to-create-a-guid-uuid
function uuidv4(){
    return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, (c) =>
        (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
    );
}

// --- Console Override Start ---
(function() {
    if (typeof window.console === 'undefined') {
        window.console = {}; // Create console if it doesn't exist
    }

    // 保存原始 console 方法到全局变量
    window._originalConsole.debug = window.console.debug || function() {};
    window._originalConsole.info = window.console.info || function() {};
    window._originalConsole.log = window.console.log || function() {};
    window._originalConsole.warn = window.console.warn || function() {};
    window._originalConsole.error = window.console.error || function() {};

    // 安全的 Go 调用函数，只有在 native bridge 可用时才调用
    function safeGoCall(funcName, args) {
        if (typeof window._runtime_invoke === 'function') {
            try {
                goCall(funcName, args, false);
            } catch (e) {
                // 静默处理错误，避免影响正常的 console 输出
                // 可以选择性地输出到原始 console，但不要循环调用
            }
        }
        // 如果 native bridge 不可用，就静默忽略，只保留原始 console 输出
    }

    window.console.log = function(...args) {
        // 始终调用原始 console.log，确保开发者工具中能看到输出
        window._originalConsole.log.apply(window.console, args);
        // 尝试发送到 Go 端，但不影响原始功能
        safeGoCall('_js_console_log', args);
    };

    window.console.debug = function(...args) {
        window._originalConsole.debug.apply(window.console, args);
        safeGoCall('_js_console_debug', args);
    };
    
    window.console.info = function(...args) {
        window._originalConsole.info.apply(window.console, args);
        safeGoCall('_js_console_info', args);
    };

    window.console.warn = function(...args) {
        window._originalConsole.warn.apply(window.console, args);
        safeGoCall('_js_console_warn', args);
    };

    window.console.error = function(...args) {
        window._originalConsole.error.apply(window.console, args);
        safeGoCall('_js_console_error', args);
    };

})();
// --- Console Override End ---