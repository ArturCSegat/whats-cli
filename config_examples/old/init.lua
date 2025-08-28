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
		local termWidth = tonumber(info["width"]) or 120

		-- Format timestamp [HH:MM]
		local iso = tostring(msg["timestamp"] or "")
		local h, min = iso:match("T(%d+):(%d+)")
		local timestamp = "[" .. (h or "??") .. ":" .. (min or "??") .. "]"

		-- Name
		local name = fromMe and "You" or (tostring(info["name"] or "???"))

		-- Compose indicators as separate styled parts
		local indicators = {}

		-- [IS QUOTE]: inverted black fg, white bg (applies only to this tag)
		if msg["isQuote"] then
			table.insert(indicators, invert_colors_of_text("[IS QUOTE]"))
		end

		-- [MEDIA], [DELETED], [VIS ONCE]: all colored with hyperlink fg/bg if set
		local function hyperlink_tag(text)
			if styles.hyperlink and styles.hyperlink.fg and styles.hyperlink.bg then
				return fg(styles.hyperlink.fg) .. bg(styles.hyperlink.bg) .. text .. reset()
			else
				return text
			end
		end

		local indicator_body = ""
		if msg["type"] == "revoked" then
			table.insert(indicators, hyperlink_tag("[DELETED]"))
			indicator_body = ""
		elseif msg["type"] == "ciphertext" then
			table.insert(indicators, hyperlink_tag("[VIS ONCE]"))
			indicator_body = ""
		elseif msg["hasMedia"] then
			table.insert(indicators, hyperlink_tag("[MEDIA]"))
			indicator_body = tostring(msg["body"] or "")
		else
			indicator_body = tostring(msg["body"] or "")
		end

		-- Build all segments separately for each line
		local prefix = timestamp .. " <" .. name .. ">: "
		local indicator_str = table.concat(indicators, "")
		local prefixLen = #strip_ansi(prefix)
		local indicatorLen = #strip_ansi(indicator_str)
		local first_wrapWidth = termWidth - prefixLen - indicatorLen
		local wrapWidth = termWidth - prefixLen
		if first_wrapWidth < 10 then first_wrapWidth = 10 end
		if wrapWidth < 10 then wrapWidth = 10 end

		-- Line wrapping for the body only: first line is shorter (for indicators)
		local words = {}
		for word in indicator_body:gmatch("%S+") do table.insert(words, word) end
		local lines, line, cur_width, is_first = {}, "", 0, true
		local max_width = first_wrapWidth
		for i, word in ipairs(words) do
			if #strip_ansi(line) + #strip_ansi(word) + 1 > max_width then
				table.insert(lines, line:match("^%s*(.-)%s*$"))
				line = word .. " "
				is_first = false
				max_width = wrapWidth
			else
				line = line .. word .. " "
			end
		end
		if #line > 0 or #indicator_body == 0 then
			table.insert(lines, line:match("^%s*(.-)%s*$"))
		end

		-- Compose output lines, only first line gets indicators
		local out_lines = {}
		for i, l in ipairs(lines) do
			local prefix_part = (i == 1) and prefix or string.rep(" ", prefixLen)
			local indicator_part = (i == 1) and indicator_str or ""
			local body_part = l
			if fromMe and styles.selfBody and styles.selfBody.fg then
				local fgcode = fg(styles.selfBody.fg)
				local bgcode = styles.selfBody.bg and bg(styles.selfBody.bg) or ""
				local resetcode = reset()
				body_part = fgcode .. bgcode .. l .. resetcode
			end
			local full_line = prefix_part .. indicator_part .. body_part
			full_line = full_line .. string.rep(" ", termWidth - #strip_ansi(full_line))
			table.insert(out_lines, full_line)
		end

		local out = table.concat(out_lines, "\n")

		-- Selection highlighting (strip ansi and invert)
		if info["is_selected"] then
			local sel_lines = {}
			for line in out:gmatch("([^\n]*)\n?") do
				if line ~= "" then
					table.insert(sel_lines, invert_colors_of_text(strip_ansi(line)))
				end
			end
			out = table.concat(sel_lines, "\n")
		end

		return out
	end,

	["chat"] = function(tbl)
		if tbl['info']['is_selected'] then
			return fg(styles.selectedStyle.fg) .. "> " .. tbl['chat']['name'] .. reset() .. "\n"
		end
		return fg(styles.unselectedStyle.fg) .. "  " .. tbl['chat']['name'] .. reset() .. "\n"
	end
}
