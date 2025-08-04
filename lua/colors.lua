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

function invert_colors(text)
	return "\27[7m" .. text .. "\27[0m"
end



styles = {
  selectedStyle = {
    fg = "10",
    bold = true
  },

  unselectedStyle = {
    fg = "8"
  },

  hyperlink = {
    fg = "#FFFFFF",
    bg = "#0000FF"
  },

  selfPrefix = {
    fg = "10"
  },

  selfBody = {
    fg = "#7FFF7F"
  },

  topbarStyke = {
    fg = "15",
    bg = "8",
    bold = true
  },

  bottombarStyle = {
    fg = "15",
    bg = "8"
  },

  replyHighlight = {
    fg = "#000000",
    bg = "#FFFFFF"
  },

  errorBarStyle = {
    fg = "#FFFFFF",
    bg = "#FF0000"
  }
}
--
