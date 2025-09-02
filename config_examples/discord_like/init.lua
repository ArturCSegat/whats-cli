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
		local fromMe      = msg["fromMe"]
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local name        = tostring(info["name"] or "")
		local headerSpace = tonumber(info["header_height"]) or 2

		local iso_timestamp   = tostring(msg["timestamp"] or "")
		local year, month, day, hour, min = iso_timestamp:match("(%d+)%-(%d+)%-(%d+)T(%d+):(%d+)")
		local timestamp = string.format("%s:%s - %s/%s", hour, min, month, day)



		-- Compose the colored name
		if name ~= "" then
			if fromMe then
				if styles.selfBody and styles.selfBody.fg then
					name = fg(styles.selfBody.fg) .. name .. reset()
				end
			else
				if styles.hyperlink and styles.hyperlink.bg then
					name = fg(styles.hyperlink.bg) .. name .. reset()
				end
			end
		end

		-- Compose the header: [selected indicator] Name  Timestamp
		local header = ""
		if selected then header = "▶ " end
		header = header .. name
		if timestamp ~= "" then
			if name ~= "" then header = header .. "  " end
			header = header .. timestamp
		end

		-- Compose the rest of the message (uncolored)
		local body = tostring(msg_table["message"]["body"] or "")
		local types = {
			["chat"] = true,
			["ptt"] = true,
			["image"] = true,
			["video"] = true,
			["audio"] = true,
			["voice"] = true,
			["document"] = true,
			["sticker"] = true,
			["contact"] = true,
			["revoked"] = true,
			["ciphertext"] = true,
		}
		if not types[msg_table["message"]['type']] then
			body = "msg of type(" .. tostring(msg_table["message"]['type']) .. ") is not properly displayed"
		end
		if msg_table["message"]["hasMedia"] then
			body = "[MEDIA]\n" .. body
		end
		if msg_table["message"]["type"] == "revoked" then
			body = "[DELETED]"
		end
		if msg_table["message"]["type"] == "ciphertext" then
			body = "[VIS ONCE]"
		end
		if msg_table["message"]["type"] == "ptt" then
			body = "[VOICE AUDIO]"
		end

		local lines = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		-- Build the message as an array of lines
		local rendered = {}
		for i = 1, headerSpace do
			table.insert(rendered, "")
		end
		table.insert(rendered, header)
		for _, line in ipairs(lines) do
			table.insert(rendered, line)
		end
		table.insert(rendered, string.rep("-", termWidth))

		return table.concat(rendered, "\n")
	end,


	["chat"] = function(tbl)
		if tbl['info']['is_selected'] then
			return fg(styles.selectedStyle.fg) .. "> " .. tbl['chat']['name'] .. reset() .. "\n"
		end
		return fg(styles.unselectedStyle.fg) .. "  " .. tbl['chat']['name'] .. reset() .. "\n"
	end
}
