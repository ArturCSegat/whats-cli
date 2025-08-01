package main

import (
	"github.com/charmbracelet/lipgloss"
	lua "github.com/yuin/gopher-lua"
)

func styleFromLuaTable(tbl *lua.LTable) lipgloss.Style {
	style := lipgloss.NewStyle()

	tbl.ForEach(func(key, value lua.LValue) {
		switch key.String() {
		case "fg":
			style = style.Foreground(lipgloss.Color(value.String()))
		case "bg":
			style = style.Background(lipgloss.Color(value.String()))
		case "bold":
			if lua.LVAsBool(value) {
				style = style.Bold(true)
			}
		case "italic":
			if lua.LVAsBool(value) {
				style = style.Italic(true)
			}
		case "underline":
			if lua.LVAsBool(value) {
				style = style.Underline(true)
			}
		}
	})

	return style
}

var styles map[string]lipgloss.Style
func setup_styles(L *lua.LState) {
	styles = make(map[string]lipgloss.Style)
	stylesTable := L.GetGlobal("styles")
	if tbl, ok := stylesTable.(*lua.LTable); ok {
		tbl.ForEach(func(key lua.LValue, value lua.LValue) {
			if subtbl, ok := value.(*lua.LTable); ok {
				name := key.String()
				style := styleFromLuaTable(subtbl)
				styles[name] = style
			}
		})
	}
}

// var (
// 	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
// 	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
// 	hyperlink       = lipgloss.NewStyle().Background(lipgloss.Color("#0000FF")).Foreground(lipgloss.Color("#FFFFFF"))
// 	selfPrefix      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
// 	selfBody        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7FFF7F"))
// 	topbarStyle     = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15")).Bold(true)
// 	bottombarStyle  = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15"))
// 	replyHighlight  = lipgloss.NewStyle().Background(lipgloss.Color("#FFFFFF")).Foreground(lipgloss.Color("#000000"))
// 	errorBarStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#FF0000")).Foreground(lipgloss.Color("#FFFFFF"))
// )
