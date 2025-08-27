function strip_ansi(str)
  -- Remove ANSI escape sequences
  str = str:gsub('\27%[[%d;?]*[ -/]*[@-~]', '') -- CSI sequences
  str = str:gsub('\27%][^%a]*%a', '')           -- OSC sequences (simplified)
  str = str:gsub('\27%]%d+;.-\7', '')           -- OSC terminated by BEL
  str = str:gsub('\27%]%d+;.-\27\\', '')        -- OSC terminated by ST (ESC\)
  str = str:gsub('\27[PX^_].-\27\\', '')        -- DCS, SOS, PM, APC sequences
  return str
end



local function hex_to_rgb(hex)
	local r, g, b = hex:match("#?(%x%x)(%x%x)(%x%x)")
	return tonumber(r, 16), tonumber(g, 16), tonumber(b, 16)
end

function fg(hex)
	local r, g, b = hex_to_rgb(hex)
	return ("\27[38;2;%d;%d;%dm"):format(r, g, b)
end

function bg(hex)
	local r, g, b = hex_to_rgb(hex)
	return ("\27[48;2;%d;%d;%dm"):format(r, g, b)
end

function reset()
	return "\27[0m"
end

function fg_rgb(r, g, b)
	return ("\27[38;2;%d;%d;%dm"):format(r, g, b)
end

function bg_rgb(r,g,b)
	return ("\27[48;2;%d;%d;%dm"):format(r, g, b)
end

function invert_colors_of_text(text)
	return "\27[7m" .. text .. "\27[0m"
end



styles = {
  selectedStyle = {
    fg = "#00796B",    -- Teal
    bold = true
  },

  unselectedStyle = {
    fg = "#B0BEC5"     -- Light Gray
  },

  hyperlink = {
    fg = "#F3E5F5",    -- Light Lavender
    bg = "#7C4DFF"     -- Deep Purple
  },

  selfPrefix = {
    fg = "#26A69A"     -- Lighter Teal
  },

  selfBody = {
    fg = "#80CBC4"     -- Muted Aqua
  },

  topbarStyke = {
    fg = "#F3E5F5",    -- Light Lavender
    bg = "#512DA8",    -- Dark Purple
    bold = true
  },

  bottombarStyle = {
    fg = "#F3E5F5",    -- Light Lavender
    bg = "#9575CD"     -- Lighter Purple
  },

  replyHighlight = {
    fg = "#263238",    -- Blue Gray (dark text)
    bg = "#E0F7FA"     -- Pale Cyan
  },

  errorBarStyle = {
    fg = "#FFFFFF",    -- White
    bg = "#D32F2F"     -- Strong Red
  }
}
