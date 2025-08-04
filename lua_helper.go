package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Escape string to be a valid Lua string literal (single quotes)
func luaEscapeString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return "'" + s + "'"
}

func struct_to_lua_table[T any](thing T) (string, error) {
	bs, err := json.Marshal(thing)
	if err != nil {
		return "", err
	}

	var m interface{}
	err = json.Unmarshal(bs, &m)
	if err != nil {
		return "", err
	}

	return toLuaValue(m)
}

// Recursively convert Go value to Lua code
func toLuaValue(v interface{}) (string, error) {
	switch v := v.(type) {
	case map[string]interface{}:
		var sb strings.Builder
		sb.WriteString("{\n")
		for k, val := range v {
			valStr, err := toLuaValue(val)
			if err != nil {
				return "", err
			}
			sb.WriteString(fmt.Sprintf("[\"%s\"] = %s,\n", k, valStr))
		}
		sb.WriteString("}")
		return sb.String(), nil
	case []interface{}:
		var sb strings.Builder
		sb.WriteString("{\n")
		for i, val := range v {
			valStr, err := toLuaValue(val)
			if err != nil {
				return "", err
			}
			// Lua arrays use integer keys starting from 1
			sb.WriteString(fmt.Sprintf("[%d] = %s,\n", i+1, valStr))
		}
		sb.WriteString("}")
		return sb.String(), nil
	case string:
		return luaEscapeString(v), nil
	case float64:
		// Lua does not distinguish between int and float
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "nil", nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

