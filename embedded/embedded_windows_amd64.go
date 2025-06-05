package embedded

import _ "embed"

const name = "wvapp.dll"

//go:embed libwvapp_windows_amd64.dll
var lib []byte
