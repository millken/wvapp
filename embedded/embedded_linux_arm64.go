package embedded

import _ "embed"

const name = "libwvapp.so"

//go:embed libwvapp_linux_arm64.so
var lib []byte
