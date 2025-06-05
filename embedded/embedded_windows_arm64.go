package embedded

import _ "embed"

const name = "wvapp.dll"

//go:embed libwvapp_windows_arm64.dll
var lib []byte
