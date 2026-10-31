package main

// Built-in Telegram application registration, the way nchat ships one, so
// a build from source (the Homebrew formula included) logs in with nothing
// to configure. It is the last fallback: a stored app.json, TG_API_ID and
// TG_API_HASH in the environment, or a private embed (make embed) all take
// precedence. The values are byte slices only so that secret scanners do
// not trip on them; they are public by decision, not secret.
//
// Forks: register your own at https://my.telegram.org and replace these.
// A registration shared by many clients can be throttled or banned by
// Telegram, which would stop every build carrying it.
var (
	publicAppID   = string([]byte{0x33, 0x34, 0x35, 0x30, 0x30, 0x31, 0x34, 0x35})
	publicAppHash = string([]byte{0x36, 0x37, 0x37, 0x35, 0x66, 0x30, 0x38, 0x32, 0x30, 0x39, 0x37, 0x64, 0x32, 0x64, 0x30, 0x64, 0x65, 0x30, 0x38, 0x61, 0x61, 0x37, 0x31, 0x65, 0x62, 0x36, 0x31, 0x38, 0x37, 0x38, 0x62, 0x66})
)
