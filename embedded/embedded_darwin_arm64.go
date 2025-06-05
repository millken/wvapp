package embedded

import _ "embed"

const name = "libwvapp.dylib"

//go:embed libwvapp_darwin_arm64.dylib
var lib []byte
