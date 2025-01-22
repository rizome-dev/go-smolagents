package smolagentsgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func parse_json_blob(json_blob string) (map[string]string, error) {
	res := make(map[string]string)
	first_accolade_index := strings.Index(json_blob, "{")
	last_accolade_index := strings.LastIndex(json_blob, "}")
	json_blob = strings.ReplaceAll(json_blob[first_accolade_index:last_accolade_index+1], `\\"`, "'")
	err := json.Unmarshal([]byte(json_blob), res)
	if err == nil {
		return res, nil
	} else {
		if terr, ok := err.(*json.UnmarshalTypeError); ok {
			place := offsetToColumn([]byte(json_blob), terr.Offset)
			if json_blob[place-1:place+2] == "},\n" {
				return nil, errors.New("ValueError: JSON is invalid: you probably tried to provide multiple tool calls in one action. PROVIDE ONLY ONE TOOL CALL.")
			} else if (place-4 >= 0) && place+5 <= len([]byte(json_blob)) {
				return nil, errors.New(fmt.Sprintf(`ValueError:
The JSON blob you used is invalid due to the following error: %e.
JSON blob was: {json_blob}, decoding failed on that specific part of the blob:
'{json_blob[place - 4 : place + 5]}'.`, err, json_blob, json_blob[place-4:place+5]))
			}
		} else {
			return nil, errors.New(fmt.Sprintf("ValueError: Error in parsing the JSON blob:", err))
		}
	}
	return nil, nil
}

func offsetToColumn(input []byte, offset int64) (column int) {
	column = 1
	for _, b := range input {
		if b == '\n' {
			column = 1
		} else {
			column++
		}
	}
	return column
}
