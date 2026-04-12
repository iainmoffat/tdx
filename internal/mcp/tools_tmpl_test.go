package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestListTemplates(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")
	store := svcs.Template.Store()

	require.NoError(t, store.Save(domain.Template{
		SchemaVersion: 1,
		Name:          "alpha",
		Rows: []domain.TemplateRow{
			{ID: "r1", Hours: domain.WeekHours{Mon: 8}},
		},
	}))
	require.NoError(t, store.Save(domain.Template{
		SchemaVersion: 1,
		Name:          "beta",
		Rows: []domain.TemplateRow{
			{ID: "r2", Hours: domain.WeekHours{Tue: 4}},
		},
	}))

	handler := listTemplatesHandler(svcs)
	result, _, err := handler(context.Background(), nil, listTemplatesArgs{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var templates []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &templates))
	require.Len(t, templates, 2)
}

func TestGetTemplate(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")
	store := svcs.Template.Store()

	require.NoError(t, store.Save(domain.Template{
		SchemaVersion: 1,
		Name:          "test",
		Rows: []domain.TemplateRow{
			{ID: "r1", Hours: domain.WeekHours{Mon: 1}},
		},
	}))

	handler := getTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, getTemplateArgs{Name: "test"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var tmpl map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &tmpl))
	require.Equal(t, "test", tmpl["name"])
}

func TestGetTemplate_NotFound(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")

	handler := getTemplateHandler(svcs)
	result, _, err := handler(context.Background(), nil, getTemplateArgs{Name: "nonexistent"})
	require.NoError(t, err)
	require.True(t, result.IsError)
}
