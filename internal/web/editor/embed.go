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

// injectTemplateData replaces the placeholders in the HTML with actual
// template JSON data and the per-session nonce.
func injectTemplateData(html string, resp templateResponse, nonce string) (string, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	escaped := strings.ReplaceAll(string(data), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	out := strings.Replace(html, `"__TEMPLATE_JSON__"`, `"`+escaped+`"`, 1)
	out = strings.Replace(out, `__TDX_SESSION__`, htmlAttrEscape(nonce), 1)
	return out, nil
}

// htmlAttrEscape escapes a string for use inside a double-quoted HTML
// attribute. The nonce is base64-RawURL (A-Za-z0-9_-) so no replacements
// occur in practice, but the helper exists to make the safety property
// explicit and future-proof.
func htmlAttrEscape(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	return s
}
