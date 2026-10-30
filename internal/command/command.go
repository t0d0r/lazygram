// Package command parses the slash-command layer of the input line.
package command

import (
	"fmt"
	"strings"
)

// Kind is the action a command requests.
type Kind int

// Command kinds.
const (
	Help Kind = iota
	Quit
	Refresh
	Chats   // list chats, optionally filtered by Arg
	Filter  // filter the feed by Arg; empty clears
	Select  // select chat Arg
	All     // clear chat selection and filter
	History // open history for chat Arg, or the selected chat
	Search  // open history search for Arg
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
	"chats": Chats, "ls": Chats, "list": Chats,
	"filter": Filter, "f": Filter, "grep": Filter,
	"select": Select, "join": Select, "j": Select, "chat": Select, "s": Select,
	"all": All, "a": All,
	"history": History, "hist": History, "open": History,
	"search": Search, "find": Search,
}

// needsArg lists kinds that cannot run without an argument.
var needsArg = map[Kind]bool{Select: true, Search: true}

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
	arg = strings.TrimSpace(arg)
	if needsArg[kind] && arg == "" {
		return Command{}, fmt.Errorf("/%s needs an argument", name)
	}
	return Command{Kind: kind, Arg: arg}, nil
}

// HelpText lists the commands for the in-app help view.
const HelpText = `Commands
  /help, /?                 show this help
  /quit, /q                 leave lazygram
  /refresh, /sync, /r       sync with the service now
  /chats [text], /ls        list chats, optionally filtered
  /select <chat>, /j <chat> show only one chat in the feed (id, alias, prefix)
  /all                      show all chats again and clear the filter
  /filter [text]            filter the feed by text or sender; empty clears
  /history [chat], /open    open full persisted history (pager)
  /search <text>            open history filtered by text`
