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
	Help    Kind = iota
	Quit         // leave
	Refresh      // sync now
	List         // print the chats; Arg filters
	Win          // switch or open a window: Arg is a number or a chat name; empty lists windows
	Close        // close window Arg (number), or the current one
	Last         // fetch the last N messages for the current window's chat
	Search       // open the local history pager; Arg is the search text
	Stats        // print client statistics into the current window
	Whois        // print what is known about a chat (Arg) or the current window's chat
	Me           // send an action line ("* text") to the current window's chat
	Press        // press a bot button: Arg is "N" or "<message id> N"
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

// HelpText lists the commands for /help. Lines stay under 70 columns so
// they fit an 80-column window after the notice indent.
const HelpText = `Commands
  /help, /?              this help
  /quit, /q              leave lazygram
  /refresh, /r           sync now
  /list [text]           chat list: @people and #channels
  /win, /win list        list of the open windows
  /win N                 switch to window N (1 is the server window)
  /win @user | #chan     open or switch to that chat's window
  /close [N]             close a window
  /last [N]              fetch the last N messages here (default 10)
  /search [text]         page through this chat's local history
  /whois [name]          what is known about a chat
  /stats                 client statistics
  /me <text>             send an action line
  /press N | <msg> N     press a bot button (numbers shown under it)`
