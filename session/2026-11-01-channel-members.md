# 2026-11-01: group and channel members

## Done

- In a group or channel window Tab completes that chat's members: after
  `/whois` (the whole argument, so a display name with spaces works) and
  in message text and `/me` text. Chat names are no longer offered
  there; a `#` in front of the `/whois` argument still completes chats.
  `/win`, `/msg` and every other command keep completing all chats in
  every window, and the server window and person windows are unchanged.
- `/whois <name>` in such a window looks the name up among the members
  first (username or display name: exact, unique prefix, substring, either
  script) and prints the username, how many of the known messages are
  theirs, the last one, and their private chat with its window. No member
  matching falls back to the chats; several is an error. `/whois` without
  a name adds the member count for a group or channel.
- Members are the senders of the messages lazygram holds for the chat
  (session and cached backlog): `domain.Members` and
  `domain.ResolveMember` (internal/domain/members.go). Own messages and
  unsigned channel posts are left out; one sender id is one member.
- `domain.Message` gained `SenderID` and `SenderNick` (optional JSON, old
  cache rows still decode). The Telegram connector fills them from
  `from_id` and the users that come with the message; the demo gives its
  group senders nicks.
- A review of the change found and fixed: list filters and the `/search`
  query in a group window had lost chat completion; display names with
  spaces could not be completed in message text; with the cursor at the
  start of message text chat names were offered; members without a
  username were completed as `@name`, a mention of whoever holds that
  username; a dropped username stayed; `/whois @nick` was ambiguous when
  another member's name equals that nick; a member with a known id could
  be given someone else's private chat by name; people named `me` or
  `channel` were mistaken for own messages and unsigned posts; the member
  count showed on every `/whois` of a group. Each has a test.
- A first run on a real account showed two gaps, both fixed: every member
  had "no username", because cached messages never took the sender id and
  username of a copy fetched again (`Store.Add` ignores known rows and
  `domain.Merge` keeps the first copy); and the second-Tab list ran
  multi-word names together. `Store.Identify` now writes the two fields
  into the cached copy (nothing else), the copy in memory follows, and
  the member list is comma-separated.
- Tab completion matches in Cyrillic or Latin (`domain.PrefixMatch`):
  chat names everywhere and members in a group window. `/whois` gained an
  either-script prefix tier after the plain prefix tier, so it resolves
  what Tab completes without making plain matches ambiguous.
- A second review found and fixed: a failed cache update hid the
  messages a window fetch had just stored (the handler now shows them
  first, as a sync does, and `Store.Identify` skips rows it cannot open);
  a common prefix of usernames could replace a name typed in another
  script (a prefix now extends the word only when it still starts with
  it, compared in transliteration, which also lets `zhiv` extend to a
  Cyrillic prefix); the either-script `/whois` tiers now come after the
  plain substring and skip queries of combining marks only.
- The member `/whois` names the group and the private chat as `/names`
  shows chats (by title in name mode), like the window bar.
- Completion now always fills in `@username`, as people mention each
  other in Telegram; filling in a display name for members without a
  username was the wrong approach, since that text mentions nobody. The
  nicks of a chat are its senders' usernames and the `@usernames` its
  messages mention (`domain.MentionsIn`), so someone who is greeted but
  never writes is offered too. Members without a username are not
  offered; `/whois Name` still finds them, and a nick only mentioned has
  its own `/whois` text.
- A third review found and fixed: `@` in links (youtube.com/@name,
  medium.com/@writer) and after non-Latin letters (an e-mail with a
  Cyrillic local part) was read as a mention; a channel signing its posts
  with its own username became its own member and hid `/whois` of the
  channel in its window (dialogs now carry `Chat.Username`); a username
  held by two people over time gave one Tab candidate that `/whois` found
  ambiguous and mention counts went to either at random (the newest
  holder keeps it); a multi-word name of someone without a username let
  the last word complete to another person's nick; each Tab rescanned all
  messages once per word (now one scan, at most four word starts); two
  hints still said Tab lists every member.
- On a real group only three nicks are offered: the members whose
  username lazygram has seen. People without a Telegram username cannot
  be mentioned by `@` at all; Telegram mentions them by name with a
  message entity, which lazygram does not send yet. Senders known only
  from old cached messages get their usernames when those messages are
  fetched again (`/last 100`). Both are todo items.
- A group window's first show fetches the last 100 messages instead of 10
  while its newest 100 known messages include one without a sender id,
  so the usernames of senders known only from the old cache are learned
  without a manual `/last 100`. Channels keep 10.
- Tab after `/whois` offers every member: `@nick`, and the display name
  for those without a username (a lookup sends nothing, so no mention can
  go wrong). Message text still gets `@nick` only.
- A group's member list now comes from Telegram when its window is first
  shown in a session: messages.getFullChat for a basic group,
  channels.getParticipants (recent first, 200 a page, up to 1000) for a
  supergroup, through the optional `provider.MemberLister`. It is merged
  into the members seen in messages (`domain.WithParticipants`) and kept
  in memory only. Any member can read a group's list unless the admins
  hid it; then, and for broadcast channels, the members seen remain.
  The earlier spec sentence that a large group's list needs admin rights
  was wrong and is corrected.
- The `[read]` and `[unread]` buttons of `/list` and `/win` now tell the
  service once per press, whatever `/read-receipts` says: `[read]` sends a
  read receipt and clears the chat's unread mark, `[unread]` sets the mark
  (`Provider.MarkUnread`, messages.markDialogUnread). Before, they only
  changed lazygram's counts unless receipts were on, and even then a
  `/list` row without a window was never reported, so the phone kept the
  messages unread. `[read]` on a chat with no message held reports up to
  the dialog's top message. `/mark-as-read` clears the mark too. Decision:
  pressing the button is an explicit request, like `/mark-as-read`; the
  global setting stays for the automatic report on focus.
- A review of the member list found and fixed: a supergroup with hidden
  members answers a non-admin with its admins and bots, not an error, so
  channels.getFullChannel now decides hidden first; muted members
  (restricted, not left) were dropped; two listed people with one name
  collapsed into one, and one person seen by name and by mention became
  two; mentions of a nick the list assigns by id were lost; a notice in
  the server window did not name its group; channels never showed as
  hidden in `/whois`.
- The server window shows only the messages that arrive during the
  session (sync, and fetched messages dated after the start). The
  cached backlog in window 0 was never a decision, only a side effect of
  every window reading one in-memory list; it filled the start screen.
- A review of the one-time reports found and fixed: `[read]` used the
  newest cached message as the limit, often older than what is unread at
  the service, so it read nothing (it now covers the service's newest);
  Telegram's unread mark was never read back, so a chat marked unread
  showed as read after a refresh (a marked chat now counts 1 unread);
  overlapping presses could reach the service out of order (requests per
  chat now go one at a time, the last press last); a failed receipt
  skipped clearing the mark, and a failed mark was reported as a failed
  receipt; `/read-receipts off` still said nothing is ever reported.
- `/mark-as-read` is the only spelling; the `/markread` alias is gone
  from the parser, `/help` and specs/ui.md.
- Mouse: a double click copies the word under the pointer, a triple
  click the screen row, Shift-click extends the last selection
  (internal/tui/selection.go; specs/ui.md, Selection and clipboard). A
  word is a run of non-blank cells with brackets, quotes and trailing
  punctuation trimmed; cells are grapheme clusters. Clickable text acts
  on the first click only. A review found and fixed: Shift-click inside
  a word or row cut it (it now anchors at the far end), and a screen
  change between a press and its release copied text from the new
  screen (the copy is now dropped). Built on branch
  `feature/mouse-word-select` and merged here.

## Decisions

- Members come from the group's member list where the account may read
  it (any member, unless the admins hid the members; never a broadcast
  channel's subscribers), plus the senders seen and the usernames
  mentioned. An earlier version of this note said no participant-list
  call would be made; that is superseded. `/last N` fetches more history
  and with it more names.
- A member completes as `@username` (a real Telegram mention) and only
  then; a mention by name for members without a username needs a message
  entity on send, left as a todo item.
- Mentions are parsed from the message text (an `@`, a letter, 3 to 31
  more letters, digits or underscores, not inside a word), not from
  Telegram's message entities, which lazygram does not keep.
- `/whois` tiers: exact name before exact username, swapped when the
  query starts with `@`, so every completion candidate resolves to its
  member.
- `/mark-as-read` stays, not `/markread`: it is the name the specs, docs
  and notices already use.

## Files touched

- internal/domain/domain.go, members.go, members_test.go
- internal/provider/telegram/history.go, history_test.go
- internal/provider/demo/demo.go, demo_test.go
- internal/store/store.go, store_test.go; internal/domain/translit.go,
  translit_test.go; internal/provider/provider.go,
  internal/provider/telegram/telegram.go, gotd.go; specs/security.md
- internal/tui/members.go, members_test.go, model.go, receipts_test.go
- internal/command/command.go, command_test.go
- internal/tui/listactions.go, receipts.go, selection.go, selection_test.go,
  clipboard.go; go.mod (uniseg direct)
- specs/ui.md (Windows, Members, completion, keys, commands, Read
  receipts, Selection and clipboard), specs/providers.md,
  specs/storage.md, specs/overview.md, docs/usage.md, docs/clipboard.md,
  README.md, todo/todo.md

## Verify

`make check`, `go test -race ./internal/...`, `make smoke`. In
`lazygram demo`: `/win #dev` (`member list: 5 members`), type `thanks @a`
and Tab twice (lists `@ada, @alan`), `/whois @` Tab twice (every member,
barbara without a username last), `/whois margaret_h` (listed, never
wrote); `/list`, Right, Enter on a row (`reported ... read`); double
click a word, triple click a row.

## State and next

- master holds the channel work, the mouse selection (from
  `feature/mouse-word-select`), their merge and this note, fast-forwarded
  from `feature/channel-nicks`; `make check`, the race tests and the
  smoke test pass on it. It is not pushed: master is ahead of
  origin/master.
- Open: a mention by name for members without a username (todo);
  whether the server window should mark where the session starts is
  moot now that it shows only this session.
- Next: push master when approved; then a patch release (v0.4.21) with
  the Homebrew formula and the Arch recipe, if wanted.
