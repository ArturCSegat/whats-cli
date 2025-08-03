package main

import (
	"encoding/json"
	"strings"
)

func struct_to_lua_table[T any](thing T) (string, error) {
	bs, err := json.Marshal(thing)
	if err != nil {
		return "", err
	}

	var m map[string]any
	err = json.Unmarshal(bs, &m)
	if err != nil {
		v := string(bs)
		v = strings.ReplaceAll(v, "[", "{")
		v = strings.ReplaceAll(v, "]", "}")
		return v, nil
	}
	mm := make(map[string]string)
	for key, val := range m {
		switch val.(type) {
		case string:
			mm[key] = val.(string)
		default:
			str, err := struct_to_lua_table(val)
			if err != nil {
				return "", err
			}
			mm[key] = str
		}

	}

	var table strings.Builder
	table.Write([]byte(" {\n"))
	for key, val := range mm {
		var val_str string
		t := strings.Trim(val, " ")
		if strings.HasPrefix(t, "{") || t == "false" || t == "true" {
			val_str = val
		} else {
			val_str = "'" + val + "'"
		}

		table.Write([]byte("[\"" + key + "\"] = " + val_str + ",\n"))
	}
	table.Write([]byte("}\n"))
	return table.String(), nil
}
