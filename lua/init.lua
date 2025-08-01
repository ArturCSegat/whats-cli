message_keybinds = {
	["up"] = function() scroll_up() end,
	["down"] = function() scroll_down() end,
	["ctrl+c"] = function() quit() end,
	["esc"] = function() escape() end,
	["enter"] = function()
		if in_input() then
			submit_input()
		else
			jump_to_quoted()
		end
	end,
	["backspace"] = function() backspace_input() end,
	["r"] = function() toggle_reply() end,
	["m"] = function() open_media() end,
	["f"] = function() forward_selected() end,
	["d"] = function() delete_selected() end,
}

chat_keybinds = {
	["up"] = function() chat_scroll_up() end,
	["down"] = function() chat_scroll_down() end,
	["k"] = function() chat_scroll_up() end,
	["j"] = function() chat_scroll_down() end,
	["ctrl+c"] = function() chat_escape() end,
	["esc"] = function() chat_escape() end,
	["enter"] = function() chat_select() end,
}
styles = {
  selectedStyle = {
    fg = "#880000", 
    bold = true
  },

  unselectedStyle = {
    fg = "#AA00AA" -- purple
  },

  hyperlink = {
    fg = "#FFFFFF",
    bg = "#880088" -- dark purple
  },

  selfPrefix = {
    fg = "#DD00DD" -- bright purple
  },

  selfBody = {
    fg = "#FFFFFF",
    bg = "#880000", -- dark red
    bold = true
  },

  topbarStyle = {
    fg = "#FFFFFF",
    bg = "#880000", -- dark red
    bold = true
  },

  bottombarStyle = {
    fg = "#FFFFFF",
    bg = "#660000" -- deeper red
  },

  replyHighlight = {
    fg = "#000000",
    bg = "#FFFFFF"
  },

  errorBarStyle = {
    fg = "#FFFFFF",
    bg = "#FF0000" -- bright red
  }
}


-- styles = {
--   selectedStyle = {
--     fg = "10",
--     bold = true
--   },
--
--   unselectedStyle = {
--     fg = "8"
--   },
--
--   hyperlink = {
--     fg = "#FFFFFF",
--     bg = "#0000FF"
--   },
--
--   selfPrefix = {
--     fg = "10"
--   },
--
--   selfBody = {
--     fg = "#7FFF7F"
--   },
--
--   topbarStyke = {
--     fg = "15",
--     bg = "8",
--     bold = true
--   },
--
--   bottombarStyle = {
--     fg = "15",
--     bg = "8"
--   },
--
--   replyHighlight = {
--     fg = "#000000",
--     bg = "#FFFFFF"
--   },
--
--   errorBarStyle = {
--     fg = "#FFFFFF",
--     bg = "#FF0000"
--   }
-- }


