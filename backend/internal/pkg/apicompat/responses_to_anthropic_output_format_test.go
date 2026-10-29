package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const outputFormatTestSchema = `{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`

func TestResponsesToAnthropicRequest_TextFormatJSONSchema(t *testing.T) {
	req := &ResponsesRequest{
		Model: "claude-sonnet-4-6",
		Input: json.RawMessage(`"Return JSON"`),
		Text:  &ResponsesText{Format: json.RawMessage(`{"type":"json_schema","name":"answer","schema":` + outputFormatTestSchema + `,"strict":true}`)},
	}

	out, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.NotNil(t, out.OutputConfig)
	assert.JSONEq(t, `{"type":"json_schema","schema":`+outputFormatTestSchema+`}`, string(out.OutputConfig.Format))

	body, err := json.Marshal(out)
	require.NoError(t, err)
	assert.JSONEq(t, outputFormatTestSchema, gjson.GetBytes(body, "output_config.format.schema").Raw)
	assert.Equal(t, "json_schema", gjson.GetBytes(body, "output_config.format.type").String())
	assert.False(t, gjson.GetBytes(body, "output_config.format.name").Exists())
	assert.False(t, gjson.GetBytes(body, "output_config.format.strict").Exists())
	assert.False(t, gjson.GetBytes(body, "output_format").Exists(), "deprecated output_format must not be emitted")
}

func TestResponsesToAnthropicRequest_TextFormatKeptWithEffort(t *testing.T) {
	format := json.RawMessage(`{"type":"json_schema","name":"answer","schema":` + outputFormatTestSchema + `}`)
	for _, model := range []string{"claude-sonnet-4-6", "claude-opus-5-5"} {
		t.Run(model, func(t *testing.T) {
			out, err := ResponsesToAnthropicRequest(&ResponsesRequest{
				Model:     model,
				Input:     json.RawMessage(`"Return JSON"`),
				Reasoning: &ResponsesReasoning{Effort: "high"},
				Text:      &ResponsesText{Format: format},
			})
			require.NoError(t, err)
			require.NotNil(t, out.OutputConfig)
			assert.Equal(t, "high", out.OutputConfig.Effort)
			assert.JSONEq(t, outputFormatTestSchema, gjson.GetBytes(out.OutputConfig.Format, "schema").Raw)
		})
	}
}

func TestResponsesToAnthropicRequest_TextFormatWithoutAnthropicEquivalent(t *testing.T) {
	for _, format := range []string{
		`{"type":"json_object"}`,
		`{"type":"text"}`,
		`{"type":"json_schema","name":"missing_schema"}`,
	} {
		out, err := ResponsesToAnthropicRequest(&ResponsesRequest{
			Model: "claude-sonnet-4-6",
			Input: json.RawMessage(`"hi"`),
			Text:  &ResponsesText{Format: json.RawMessage(format)},
		})
		require.NoError(t, err)
		assert.Nil(t, out.OutputConfig, format)
	}
}

func TestChatCompletionsResponseFormatToAnthropicOutputFormat(t *testing.T) {
	ccReq := &ChatCompletionsRequest{
		Model:          "claude-sonnet-4-6",
		Messages:       []ChatMessage{{Role: "user", Content: json.RawMessage(`"Return JSON"`)}},
		ResponseFormat: json.RawMessage(`{"type":"json_schema","json_schema":{"name":"answer","schema":` + outputFormatTestSchema + `,"strict":true}}`),
	}

	responsesReq, err := ChatCompletionsToResponses(ccReq)
	require.NoError(t, err)
	out, err := ResponsesToAnthropicRequest(responsesReq)
	require.NoError(t, err)

	body, err := json.Marshal(out)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"json_schema","schema":`+outputFormatTestSchema+`}`, gjson.GetBytes(body, "output_config.format").Raw)
}
