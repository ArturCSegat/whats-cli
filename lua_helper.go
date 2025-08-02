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
		return "", err
	}
	mm := make(map[string]string)
	for key, val := range m {
		switch val.(type) {
		case string:
			mm[key] = val.(string)
		default:
			bs, err = json.Marshal(val)
			if err != nil {
				return "", err
			}
			mm[key] = string(bs)
		}

	}

	var table strings.Builder
	table.Write([]byte(" {\n"))
	for key, val := range mm {
		table.Write([]byte("[\"" + key + "\"] = '" + val + "',\n"))
	}
	table.Write([]byte("}\n"))
	return table.String(), nil
}

