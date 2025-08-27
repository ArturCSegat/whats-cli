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
	["ctrl+a"] = function() append_input("porrafodase") end,
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

-- hooks = {
-- 	["onMsg"] = function(table)
-- 		if io.open then
-- 			local file = io.open("./out/output.txt", "a")
--
-- 			if file then
-- 				file:write("msg from(" .. table["from"] .. "): " .. table["body"])
-- 				file:write("\n")
-- 				file:close()
-- 			else
-- 				print("Failed to open file for writing.")
-- 			end
-- 		else
-- 			print("Os functions unavailable")
-- 		end
-- 	end
--
-- }

renders = {
	["message1"] = function(msg_table)
		local msg         = msg_table["message"]
		local info        = msg_table["info"] or {}
		local body        = tostring(msg["body"] or "")
		local name        = tostring(info["name"] or "")
		local fromMe      = msg["fromMe"]
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local headerSpace = tonumber(info["header_height"]) or 1

		-- Split lines
		local lines       = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		-- Measure max line width
		local contentWidth = 0
		for _, line in ipairs(lines) do
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
			topBorderContent = invert_colors(colorStart .. topBorderContent .. colorReset)
		else
			topBorderContent = colorStart .. topBorderContent .. colorReset
		end
		local topBorder = padStr .. topBorderContent

		-- Build box lines
		local bubble = {}
		table.insert(bubble, topBorder)

		for _, line in ipairs(lines) do
			local padding = boxWidth - #line - 4
			local leftSpace = math.floor(padding / 2)
			local rightSpace = padding - leftSpace
			local content = "│ " .. string.rep(" ", leftSpace) .. line .. string.rep(" ", rightSpace) .. " │"
			if selected then
				content = invert_colors(colorStart .. content .. colorReset)
			else
				content = colorStart .. content .. colorReset
			end
			table.insert(bubble, padStr .. content)
		end

		-- Bottom border
		local bottomBorderContent = "└" .. string.rep("─", boxWidth - 2) .. "┘"
		if selected then
			bottomBorderContent = invert_colors(colorStart .. bottomBorderContent .. colorReset)
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

	["message"] = function(msg_table)
		local msg         = msg_table["message"]
		local info        = msg_table["info"] or {}
		local from        = msg["from"]
		local body        = tostring(msg["body"] or "")
		local fromMe      = msg["fromMe"]
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local name        = tostring(info["name"] or "")
		local headerSpace = tonumber(info["header_height"]) or 2

		local lines       = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		local contentWidth = 0
		for _, line in ipairs(lines) do
			if #line > contentWidth then contentWidth = #line end
		end
		local bubbleWidth = contentWidth + 4

		local bubble = {}
		table.insert(bubble, "┌" .. string.rep("─", contentWidth + 2) .. "┐")
		for _, line in ipairs(lines) do
			local pad = contentWidth - #line
			table.insert(bubble, "│ " .. line .. string.rep(" ", pad) .. " │")
		end
		table.insert(bubble, "└" .. string.rep("─", contentWidth + 2) .. "┘")

		-- Apply self color if fromMe
		if fromMe and styles.selfBody and styles.selfBody.fg then
			local colorPrefix = fg(styles.selfBody.fg)
			local colorSuffix = reset()
			for i, line in ipairs(bubble) do
				bubble[i] = colorPrefix .. line .. colorSuffix
			end
		end

		-- Apply highlight if selected
		if selected then
			for i, line in ipairs(bubble) do
				bubble[i] = "\27[30;47m" .. line .. "\27[0m"
			end
		end

		local tail = fromMe and "╰─▶" or "◀─╯"
		if fromMe and styles.selfBody and styles.selfBody.fg then
			tail = fg(styles.selfBody.fg) .. tail .. reset()
		end
		table.insert(bubble, tail)

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

		for i = 1, headerSpace do
			table.insert(bubble, 1, "")
		end

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

	["message1"] = function(msg_table)
		local msg         = msg_table["message"]
		local info        = msg_table["info"] or {}
		local body        = tostring(msg["body"] or "")
		local fromMe      = msg["fromMe"]
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local headerSpace = tonumber(info["header_height"]) or 2
		local name        = tostring(info["name"] or "")

		-- Split message into lines
		local lines       = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		-- Compute max line width
		local contentWidth = 0
		for _, line in ipairs(lines) do
			if #line > contentWidth then contentWidth = #line end
		end

		-- Build pointer lines
		local rendered = {}

		-- Add top spacing
		for _ = 1, headerSpace do
			table.insert(rendered, "")
		end

		-- Add name if available
		if name ~= "" then
			local nameLine = name
			if fromMe then
				nameLine = string.rep(" ", termWidth - #name) .. name
			end
			table.insert(rendered, nameLine)
		end

		-- Prepare message lines
		for _, line in ipairs(lines) do
			if fromMe then
				local pad = termWidth - #line
				table.insert(rendered, string.rep(" ", pad) .. line)
			else
				table.insert(rendered, line)
			end
		end

		-- Add pointer line
		local pointer
		if fromMe then
			pointer = string.rep(" ", termWidth - 2) .. "\\\n" ..
			    string.rep(" ", termWidth - 3) .. "●>"
		else
			pointer = "  /\n<●"
		end
		for line in pointer:gmatch("[^\n]+") do
			table.insert(rendered, line)
		end

		-- Apply highlight if selected
		if selected then
			for i, line in ipairs(rendered) do
				rendered[i] = "\27[30;47m" .. line .. "\27[0m"
			end
		end

		return table.concat(rendered, "\n")
	end,

	["message1"] = function(msg_table)
		local msg         = msg_table["message"]
		local info        = msg_table["info"] or {}
		local body        = tostring(msg["body"] or "")
		local name        = tostring(info["name"] or "")
		local selected    = info["is_selected"]
		local termWidth   = tonumber(info["width"]) or 80
		local headerSpace = tonumber(info["header_height"]) or 1

		local lines       = {}
		for line in body:gmatch("[^\r\n]+") do
			table.insert(lines, line)
		end

		local separator = string.rep("-", termWidth)
		local output = {}

		for _ = 1, headerSpace do
			table.insert(output, "")
		end

		table.insert(output, separator)

		local style = selected and styles.selectedStyle or styles.unselectedStyle

		-- Color by participant
		local nameColor = fg(msg.fromMe and styles.selfBody.fg or styles.hyperlink.bg)
		local bold = style.bold and "\27[1m" or ""
		local styledName = nameColor .. bold .. name .. ":" .. reset()

		table.insert(output, styledName)

		for _, line in ipairs(lines) do
			table.insert(output, line)
		end

		if selected and style.bg then
			local blockFg = fg(style.fg or "#000000")
			local blockBg = bg(style.bg)
			local b = style.bold and "\27[1m" or ""
			for i, line in ipairs(output) do
				output[i] = blockBg .. blockFg .. b .. line .. reset()
			end
		end

		return table.concat(output, "\n")
	end
}
