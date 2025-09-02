# `init.lua` – WhatsApp CLI Configuration Overview

This file configures keybindings and message rendering for a WhatsApp command-line interface (CLI). It enables customization of how you interact with chats and how messages are displayed in the terminal.

---

## Key Concepts

- **Keybind Tables**: Define keyboard shortcuts for different interface contexts (messages vs. chat list).
- **Render Table**: Controls how messages are visually displayed in the terminal, using styles and colors.
- **Hooks**: Controls behaviour on certain events.

---

### Bindable Actions

#### Messages Actions

- `"scroll_up"`
- `"scroll_down"`
- `"escape"` -> Goes back to chat list or exits input mode
- `"jump_to_quoted"` -> Jumps to the message the current selected message is quoting
- `"toggle_reply"` -> Toggles reply mode to the current selected message
- `"open_media"` -> Opens the media attached to the selected message on the default browser
- `"forward_message"` -> Forwards the selected message to another chat
- `"delete_message"` -> Deletes the selected message
- `"apend_input"` -> appends a string to the current input
- `"backspace_input"` -> Deletes the last character from the current input
- `"submit_input"` -> Submits the current input as a message
- `"quit"` -> Quits the application

#### Messages Functions

- `"in_input()"` -> Returns true if the user is currently typing a message
- `"input_content()"` -> Returns the current content of the input box
- `"current_message_tbl()"` -> Returns the lua table of the currently selected message (follows the format in the `renders["message"]` section below)
- `"current_chat_tbl()"` -> Returns the lua table of the currently opened chat (follows the format in the `renders["chats"]` section below)

#### Chats Keybind Actions

- `"chat_scroll_up"`
- `"chat_scroll_down"`
- `"chat_escape"` -> Quits the application
- `"chat_select"` -> Selects the highlighted chat and opens it

#### Chats Functions

- `"current_chat_tbl()"` -> Returns the lua table of the currently opened chat (follows the format in the `renders["chats"]` section below)


### Renders

Controls how messages are formatted and colored in the terminal.


#### `renders["message"]`
Your message renderer function will receive a message object with the following properties:

```
{
    ["message"] = {
        ["from"] = '[PHONE-NUMBER@c.us-OR-GROUPID@g.us]',
        ["type"] = 'chat',
        ["groupMemberFrom"] = '[IF-IN-GROUP-PHONE-OF-MEMBER-ELSE-EMPTY]@c.us' ,
        ["fromMe"] = true,
        ["body"] = 'lol',
        ["timestamp"] = '2025-08-22T01:27:51Z',
        ["groupInvite"] = '',
        ["quoteId"] = '',
        ["id"] = 'MESSAGE-ID-STRING',
        ["hasMedia"] = false,
        ["isQuote"] = false,
        ["isForwarded"] = false,
        ["mentionedIds"] = {
        },
        ["info"] = {
            ["read"] = false,
            ["delivered"] = false,
            ["played"] = false,
        },
    },
    ["info"] = {
            ["height"] = HEIGHT-OF-TERMINAL,
            ["name"] = '[NAME-OF-SENDER] Or You]',
            ["is_selected"] = false,
            ["width"] = WIDTH-OF-TERMINAL,
    },
}
```

#### `renders["chat"]`
Your chat renderer function will receive a chat object with the following properties:

```
{
    ["chat"] = {
        ["unreadCount"] = 0,
        ["isGroup"] = false,
        ["isMuted"] = false,
        ["isReadOnly"] = false,
        ["isPinned"] = false,
        ["id"] = '[PHONE-NUMBER@c.us-OR-GROUPID@g.us]',
        ["name"] = '[CHAT-NAME]',
        ["lastMessage"] = [MESSAGE-OBJECT-IN-THE-FORMAT-SPECIFIED-ABOVE],
        ["isArchived"] = false,
    },
    ["info"] = {
        ["width"] = WIDTH-OF-TERMINAL,
        ["height"] = HEIGHT-OF-TERMINAL,
        ["is_selected"] = true,
    },
}

```

The renderer functions should return a string that represents how the message or chat should be displayed.
The `colors.lua` file provides various color and style functions that can be used to format the output.
The chats page and the messages page will then be rendered according to the defined renderer function.
You may also call other scripts that return strings so to not depend on the lua-gopher runtime.

Ex:
```lua

local function call_python_script_named_lol_dot_py_and_return_resulting_string() 
	local handle = io.popen("python3 lol.py")
	local result = handle:read("*a")
	handle:close()
	return result
end 

renders = {
    ["message"] = function(msg)
        return call_python_script_named_lol_dot_py_and_return_resulting_string()
    end,
}
```
Note that passing the message or chat object to an external script may require serialization wich may not be trivial.


### Hooks

#### Available Hooks

- `onMsg`: Called when a new message is received. The function receives the received message in the same format as the renderer function.

