// Package command parses the slash-command layer of the input line.
package command

import (
	"fmt"
	"sort"
	"strings"
)

// Kind is the action a command requests.
type Kind int

// Command kinds.
const (
	Help     Kind = iota
	Quit          // leave
	Refresh       // sync now
	List          // print the chats; Arg filters
	Win           // switch or open a window: Arg is a number or a chat name; empty lists windows
	Close         // close window Arg (number), or the current one
	Last          // fetch the last N messages for the current window's chat
	Search        // open the local history pager; Arg is the search text
	Stats         // print client statistics into the current window
	Whois         // print what is known about a chat (Arg) or the current window's chat
	Me            // send an action line ("* text") to the current window's chat
	Press         // press a bot button: Arg is "N" or "<message id> N"
	Open          // fetch and open the attachment of the latest message with one, or of message Arg
	Theme         // list the colour themes, or switch to Arg
	Move          // move the current window to position Arg
	Msg           // send a message: Arg is "<@user|#chan> <text>"; the window opens if missing
	About         // print version, author and build information
	Fx            // toggle the running lights over clickable text
	Mouse         // toggle mouse reporting (off lets the terminal select text)
	Clip          // clipboard mode for the mouse selection: Arg is auto, osc52, tool or off; empty shows
	Edit          // edit the last own message in this window: Arg is the new text (empty: prefill)
	Delete        // delete the last own message in this window
	Nicks         // toggle showing chats by nick (@alias) or by name (@title)
	SendFile      // send a local file: Arg is "<path> [caption]"
	Redraw        // drop the notices and pictures from the window, keep the messages
	Keys          // print the keyboard shortcut reference into the server window
)

// Command is a parsed slash command.
type Command struct {
	Kind Kind
	Arg  string
}

var table = map[string]Kind{
	"help": Help, "h": Help, "?": Help,
	"quit": Quit, "q": Quit, "exit": Quit,
	"refresh": Refresh, "sync": Refresh, "r": Refresh,
	"list": List, "ls": List, "chats": List,
	"win": Win, "w": Win, "window": Win, "j": Win, "join": Win, "query": Win,
	"close": Close, "wc": Close, "part": Close,
	"last": Last, "history": Last, "hist": Last,
	"search": Search, "find": Search, "log": Search,
	"stats": Stats, "uptime": Stats,
	"whois": Whois, "wi": Whois, "info": Whois,
	"me": Me, "action": Me,
	"press": Press, "button": Press, "btn": Press,
	"open": Open, "fetch": Open, "download": Open,
	"theme": Theme, "colors": Theme, "colours": Theme,
	"move": Move, "mv": Move,
	"msg": Msg, "say": Msg, "tell": Msg,
	"about": About, "version": About,
	"fx": Fx, "lights": Fx, "anim": Fx,
	"mouse": Mouse,
	"clip":  Clip, "clipboard": Clip,
	"edit": Edit, "e": Edit,
	"delete": Delete, "del": Delete, "rm": Delete,
	"names": Nicks, "nicks": Nicks,
	"send": SendFile, "file": SendFile, "upload": SendFile,
	"redraw": Redraw, "clear": Redraw,
	"keys": Keys, "shortcuts": Keys, "bindings": Keys,
}

// Parse turns an input line into a Command. The line must start with "/".
func Parse(line string) (Command, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return Command{}, fmt.Errorf("not a command (start with /): sending messages is not supported yet")
	}
	name, arg, _ := strings.Cut(strings.TrimPrefix(line, "/"), " ")
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return Command{}, fmt.Errorf("empty command, try /help")
	}
	kind, ok := table[name]
	if !ok {
		return Command{}, fmt.Errorf("unknown command /%s, try /help", name)
	}
	return Command{Kind: kind, Arg: strings.TrimSpace(arg)}, nil
}

// Names lists every command name with its slash, sorted, for completion.
func Names() []string {
	out := make([]string, 0, len(table))
	for n := range table {
		out = append(out, "/"+n)
	}
	sort.Strings(out)
	return out
}

// HelpText lists the commands for /help: every spelling the parser table
// accepts appears as a slash-prefixed word, the short forms on an "also"
// continuation of their command (TestHelpTextCoversEveryName enforces
// it). The keys are a separate list, printed by /keys, and the last line
// says so. Lines stay under 70 columns so they fit an 80-column window
// after the notice indent.
const HelpText = `Commands
  /help, /?              this command list (F1 too); also /h
  /keys                  the keyboard shortcuts;
                         also /shortcuts, /bindings
  /quit, /q              leave lazygram; also /exit
  /refresh, /r           sync now; also /sync
  /list [text]           chat list: @people and #channels;
                         also /ls, /chats
  /win, /win list        list of the open windows
  /win N                 switch to window N (0 is the server window)
  /win @user | #chan     open or switch to that chat's window
                         /win is also /w, /window, /j, /join, /query
  /close [N]             close a window; also /wc, /part
  /move N                move this window to position N; also /mv
  /last [N]              fetch the last N messages here (default 10);
                         also /history, /hist
  /search [text]         page through this chat's local history;
                         also /find, /log
  /whois [name]          what is known about a chat; also /wi, /info
  /stats                 client statistics; also /uptime
  /me <text>             send an action line; also /action
  /msg <@user|#chan> <text>  send to that chat, opening its window;
                         also /say, /tell
  /send <path> [caption] send a local file; pictures go as photos
                         (Tab completes the path); also /file, /upload
  /redraw, /clear        drop the notices and pictures from this
                         window; the messages stay
  /edit [text]           edit your last message here
                         (no text: put it in the prompt); also /e
  /delete                delete your last message here, for everyone;
                         also /del, /rm
  /press N | <msg> N     press a bot button (numbers shown under it);
                         also /button, /btn
  /open [msg]            fetch a photo and show it in the terminal
                         (a file: decrypt a copy and show where)
  /open out [msg]        the same, but in the system viewer
                         /open is also /fetch, /download
  /theme [name]          list the colour themes, or switch to one;
                         also /colors, /colours
  /about                 version, author and build information;
                         also /version
  /fx                    running lights over links and buttons
                         on or off; also /lights, /anim
  /mouse                 mouse reporting on or off
                         (off: the terminal selects text)
  /clip [mode]           selection clipboard: auto (terminal/tool),
                         osc52, tool or off (kept); also /clipboard
  /names                 show chats by nick (@alias) or name (@Title);
                         also /nicks
Keyboard shortcuts: type /keys.`
