
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

renders = {
		["message"] = function(msg_table)
		local msg         = msg_table["message"]
		local info        = msg_table["info"] or {}
		local body        = tostring(msg["body"] or "")
		local name        = tostring(info["name"] or "")
		local fromMe      = msg["fromMe"]
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local headerSpace = tonumber(info["header_height"]) or 1

		if msg["hasMedia"] then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[MEDIA]" .. reset() .. "\n" ..  body
		end
		if msg["type"] == "revoked" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg).. "[DELETED]" .. reset()
		end
		if msg["type"] == "ciphertext" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[VIS ONCE]".. reset()
		end
		if msg["type"] == "ptt" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[VOICE AUDIO]".. reset()
		end
		-- check if type not in list of types
		local types = {
			["chat"]=true,
			["ptt"]=true,
			["image"]=true,
			["video"]=true,
			["audio"]=true,
			["voice"]=true,
			["document"]=true,
			["sticker"]=true,
			["contact"]=true,
			["revoked"]=true,
			["ciphertext"]=true,
		}
		if not types[msg['type']] then
			body =  "msg of type(" .. tostring(msg['type']) .. ") is not properly displayed"
		end

		-- Split lines
		local lines       = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		-- Measure max line width
		local contentWidth = 0
		for _, line in ipairs(lines) do
			line = strip_ansi(line)
			if #line > contentWidth then contentWidth = #line end
		end

		-- Final box width (with padding)
		local boxWidth = contentWidth + 4
		if #name + 4 > boxWidth then
			boxWidth = #name + 4
		end

		-- Calculate left padding for centering
		local leftPad = math.floor((termWidth - boxWidth) / 2)
		local padStr = string.rep(" ", leftPad)

		-- Choose outline color
		local outlineColor = fromMe and styles.selfBody.fg or styles.hyperlink.bg
		local colorStart = fg(outlineColor)
		local colorReset = reset()

		-- Prepare top border with centered name
		local leftLineLen = math.floor((boxWidth - #name) / 2) - 1
		local rightLineLen = boxWidth - #name - leftLineLen - 2

		local topBorderContent = "┌" ..
		string.rep("─", leftLineLen) .. name .. string.rep("─", rightLineLen) .. "┐"
		if selected then
			topBorderContent = invert_colors_of_text(colorStart .. topBorderContent .. colorReset)
		else
			topBorderContent = colorStart .. topBorderContent .. colorReset
		end
		local topBorder = padStr .. topBorderContent

		-- Build box lines
		local bubble = {}
		table.insert(bubble, topBorder)

		for _, line in ipairs(lines) do
			local padding = boxWidth - #strip_ansi(line) - 4
			local leftSpace = math.floor(padding / 2)
			local rightSpace = padding - leftSpace
			local content = "│ " .. string.rep(" ", leftSpace) .. line .. string.rep(" ", rightSpace) .. fg(outlineColor) .. " │" ..reset()
			if selected then
				content = invert_colors_of_text(colorStart .. strip_ansi(content) .. colorReset)
			else
				content = colorStart .. content .. colorReset
			end
			table.insert(bubble, padStr .. content)
		end

		-- Bottom border
		local bottomBorderContent = "└" .. string.rep("─", boxWidth - 2) .. "┘"
		if selected then
			bottomBorderContent = invert_colors_of_text(colorStart .. bottomBorderContent .. colorReset)
		else
			bottomBorderContent = colorStart .. bottomBorderContent .. colorReset
		end
		table.insert(bubble, padStr .. bottomBorderContent)

		-- Add vertical spacing on top
		for i = 1, headerSpace do
			table.insert(bubble, 1, "")
		end

		return table.concat(bubble, "\n")
	end,


	["chat"] = function (tbl)
		if tbl['info']['is_selected'] then
			return fg(styles.selectedStyle.fg) ..  "> " .. tbl['chat']['name'] .. reset() .. "\n"
		end
		return fg(styles.unselectedStyle.fg) ..  "  " .. tbl['chat']['name'] .. reset() .. "\n"
	end
}
