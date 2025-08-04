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
--

hooks = {
	["onMsg"] = function(table)
		if io.open then
			local file = io.open("./out/output.txt", "a")

			if file then
				file:write("msg from(" .. table["from"] .. "): " .. table["body"])
				file:write("\n")
				file:close()
			else
				print("Failed to open file for writing.")
			end
		else
			print("Os functions unavailable")
		end
	end

}

renders = {
	["message"] = function(msg_table)
		local msg         = msg_table["message"]
		local info        = msg_table["info"] or {}
		local from        = msg["from"]
		local body        = tostring(msg["body"] or "")
		local fromMe      = msg["fromMe"]
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local name        = tostring(info["name"] or "")
		local headerSpace = tonumber(info["header_height"]) or 2 -- vertical space reserved above

		-- Split message lines
		local lines       = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		-- Calculate content and bubble width
		local contentWidth = 0
		for _, line in ipairs(lines) do
			if #line > contentWidth then contentWidth = #line end
		end
		local bubbleWidth = contentWidth + 4 -- 2 for padding, 2 for borders

		-- Build chat bubble
		local bubble = {}
		table.insert(bubble, "┌" .. string.rep("─", contentWidth + 2) .. "┐")
		for _, line in ipairs(lines) do
			local pad = contentWidth - #line
			table.insert(bubble, "│ " .. line .. string.rep(" ", pad) .. " │")
		end
		table.insert(bubble, "└" .. string.rep("─", contentWidth + 2) .. "┘")

		-- Apply highlight if selected
		if selected then
			for i, line in ipairs(bubble) do
				bubble[i] = "\27[30;47m" .. line .. "\27[0m"
			end
		end

		-- Add tail
		local tail = fromMe and "╰─▶" or "◀─╯"
		table.insert(bubble, tail)

		-- Add name above bubble if present
		if name ~= "" then
			local nameLine = name
			if fromMe then
				local namePad = termWidth - #name
				if namePad > 0 then
					nameLine = string.rep(" ", namePad) .. name
				end
			end
			table.insert(bubble, 1, nameLine)
		end

		-- Reserve vertical space above
		for i = 1, headerSpace do
			table.insert(bubble, 1, "")
		end

		-- Apply right alignment (after name/space)
		if fromMe then
			local leftPad = termWidth - bubbleWidth
			if leftPad < 0 then leftPad = 0 end
			for i = headerSpace + 1, #bubble do
				bubble[i] = string.rep(" ", leftPad) .. bubble[i]
			end
		end

		return table.concat(bubble, "\n")
	end,

	["message1"] = function(msg_table)
		return "from(" .. msg_table["message"]["from"] .. "): " .. msg_table["message"]["body"]
	end,
}
