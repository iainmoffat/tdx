package editor

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed static/editor.html
var editorHTML string

// templateResponse is the JSON shape served to the browser.
type templateResponse struct {
	Name string            `json:"name"`
	Rows []templateRowJSON `json:"rows"`
}

type templateRowJSON struct {
	ID       string    `json:"id"`
	Label    string    `json:"label"`
	Group    string    `json:"group"`
	TypeName string    `json:"typeName"`
	Hours    hoursJSON `json:"hours"`
}

type hoursJSON struct {
	Sun float64 `json:"sun"`
	Mon float64 `json:"mon"`
	Tue float64 `json:"tue"`
	Wed float64 `json:"wed"`
	Thu float64 `json:"thu"`
	Fri float64 `json:"fri"`
	Sat float64 `json:"sat"`
}

// injectTemplateData replaces the placeholder in the HTML with actual
// template JSON data.
func injectTemplateData(html string, resp templateResponse) (string, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(data), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return strings.Replace(html, `"__TEMPLATE_JSON__"`, `"`+escaped+`"`, 1), nil
}
