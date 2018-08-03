package gojenkins

import "encoding/json"

func makeJson(data interface{}) string {
	str, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(json.RawMessage(str))
}
