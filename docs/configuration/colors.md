# `colors.lua`

This file provides helper functions and style definitions for coloring and formatting text in your WhatsApp CLI interface. It enables rich, readable, and visually distinct message displays using terminal escape codes.

---

## Color Functions

### 1. `hex_to_rgb(hex)`

- Converts a hexadecimal color string (e.g., `#00FF00`) into its RGB (Red, Green, Blue) components.
- Used internally by other functions.

### 2. `fg(hex)`

- Returns a terminal escape sequence to set the **foreground** (text) color using an RGB hex string.
- Example: `fg("#FF0000")` makes text red.

### 3. `bg(hex)`

- Returns a terminal escape sequence to set the **background** color using an RGB hex string.
- Example: `bg("#0000FF")` makes background blue.

### 4. `reset()`

- Returns the escape code to reset all terminal formatting (color, bold, etc.).
- Use after color codes to avoid "bleeding" styling into later text.

### 5. `invert_colors(text)`

- Wraps the provided text with the escape code for "reverse video" (swap foreground and background colors).
- Useful for highlighting selected messages.

---

## Style Table

The `styles` table defines named color and font styles used throughout the CLI. These can be referenced in rendering logic (e.g., in `init.lua`).

### Example Style Entries

- `selectedStyle`: Style for selected messages .
- `unselectedStyle`: Style for unselected messages .
- `hyperlink`: Style for links .
- `selfBody`: Used for messages sent by you .
- `topbarStyke`/`bottombarStyle`: Styles for UI bars.
- `replyHighlight`: Highlight for reply messages.
- `errorBarStyle`: For errors .

---

## Usage Example

```lua
print(fg("#FF0000") .. "This is red text" .. reset())
print(bg("#00FF00") .. "Green background" .. reset())
print(invert_colors("Inverted colors!"))
```
