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
		local msg = msg_table["message"]
		local info = msg_table["info"] or {}
		local fromMe = msg["fromMe"]
		local styles = styles or {}
		local termWidth = tonumber(info["width"]) or 80
		local headerSpace = tonumber(info["header_height"]) or 2

		-- Format timestamp
		local iso = tostring(msg["timestamp"] or "")
		local y, m, d, h, min = iso:match("(%d+)%-(%d+)%-(%d+)T(%d+):(%d+)")
		local timestamp = string.format("%s:%s - %s/%s", h or "??", min or "??", m or "??", d or "??")

		-- Prepare message body
		local body = tostring(msg["body"] or "")
		if msg["hasMedia"] then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[MEDIA]" .. reset() .. "\n" .. body
		elseif msg["type"] == "revoked" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[DELETED]" .. reset()
		elseif msg["type"] == "ciphertext" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[VIS ONCE]" .. reset()
		elseif msg["type"] == "ptt" then
			body = fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. "[VOICE AUDIO]" .. reset()
		end

		local allowed_types = {
			chat = true,
			ptt = true,
			image = true,
			video = true,
			audio = true,
			voice = true,
			document = true,
			sticker = true,
			contact = true,
			revoked = true,
			ciphertext = true
		}
		if not allowed_types[msg["type"]] then
			body = "msg of type(" .. tostring(msg['type']) .. ") is not properly displayed"
		end

		-- Split into lines and find max width
		local lines, width = {}, 0
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
			local l = #strip_ansi(line)
			if l > width then width = l end
		end

		local function style_line(line, color)
			return color and fg(color) .. line .. reset() or line
		end

		-- Build bubble
		local bubble = {}
		table.insert(bubble, "┌" .. string.rep("─", width + 2) .. "┐")
		for _, line in ipairs(lines) do
			local pad = width - #strip_ansi(line)
			table.insert(bubble, "│ " .. line .. string.rep(" ", pad) .. " │")
		end
		table.insert(bubble, "└" .. string.rep("─", width + 2) .. "┘")

		-- Style for sender
		if fromMe and styles.selfBody and styles.selfBody.fg then
			for i = 1, #bubble do bubble[i] = style_line(bubble[i], styles.selfBody.fg) end
		end
		if info["is_selected"] then
			for i = 1, #bubble do
				bubble[i] = "\27[30;47m" .. strip_ansi(bubble[i]) .. "\27[0m"
			end
		end

		-- Tail
		local tail = fromMe and "╰─▶" or "◀─╯"
		if fromMe and styles.selfBody and styles.selfBody.fg then
			tail = style_line(tail, styles.selfBody.fg)
		end
		table.insert(bubble, tail)

		-- Name + timestamp
		local name = tostring(info["name"] or "")
		if name ~= "" then
			local nt = name .. "  " .. timestamp
			if fromMe then
				nt = string.rep(" ", math.max(0, termWidth - #nt)) .. nt
			end
			if fromMe and styles.selfBody and styles.selfBody.fg then
				nt = style_line(nt, styles.selfBody.fg)
			end
			table.insert(bubble, 1, nt)
		end

		-- Header space
		for _ = 1, headerSpace do table.insert(bubble, 1, "") end

		-- Bubble alignment
		if fromMe then
			local pad = math.max(0, termWidth - (width + 4))
			for i = headerSpace + 2, #bubble do
				bubble[i] = string.rep(" ", pad) .. bubble[i]
			end
		end

		return table.concat(bubble, "\n")
	end,

	["chat"] = function(tbl)
		if tbl['info']['is_selected'] then
			return fg(styles.selectedStyle.fg) .. "> " .. tbl['chat']['name'] .. reset() .. "\n"
		end
		return fg(styles.unselectedStyle.fg) .. "  " .. tbl['chat']['name'] .. reset() .. "\n"
	end
}
