package apicompat

import "encoding/json"

func chatResponseFormatToResponsesTextFormat(raw json.RawMessage) json.RawMessage {
	raw = normalizedRawJSON(raw)
	if len(raw) == 0 {
		return nil
	}

	obj, ok := rawJSONObject(raw)
	if !ok || rawString(obj["type"]) != "json_schema" {
		return raw
	}

	schemaRaw := normalizedRawJSON(obj["json_schema"])
	if len(schemaRaw) == 0 {
		return raw
	}

	var schema map[string]json.RawMessage
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		return raw
	}
	schema["type"] = rawJSONString("json_schema")

	out, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return out
}

func responsesTextFormatToChatResponseFormat(raw json.RawMessage) json.RawMessage {
	raw = normalizedRawJSON(raw)
	if len(raw) == 0 {
		return nil
	}

	obj, ok := rawJSONObject(raw)
	if !ok || rawString(obj["type"]) != "json_schema" {
		return raw
	}
	if _, alreadyChatShape := obj["json_schema"]; alreadyChatShape {
		return raw
	}

	schema := make(map[string]json.RawMessage, len(obj))
	for key, value := range obj {
		if key == "type" {
			continue
		}
		schema[key] = value
	}
	if len(schema) == 0 {
		return raw
	}

	schemaRaw, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	out, err := json.Marshal(map[string]json.RawMessage{
		"type":        rawJSONString("json_schema"),
		"json_schema": schemaRaw,
	})
	if err != nil {
		return raw
	}
	return out
}

// responsesTextFormatToAnthropicOutputFormat maps Responses text.format
// {type:"json_schema", name, schema, strict} to Anthropic output_config.format
// {type:"json_schema", schema}. Anthropic always enforces the schema and has no
// name/strict fields; json_object/text have no Anthropic equivalent → nil.
// A Chat-shaped {type, json_schema:{schema}} is tolerated as well.
func responsesTextFormatToAnthropicOutputFormat(raw json.RawMessage) json.RawMessage {
	obj, ok := rawJSONObject(normalizedRawJSON(raw))
	if !ok || rawString(obj["type"]) != "json_schema" {
		return nil
	}
	schema := normalizedRawJSON(obj["schema"])
	if len(schema) == 0 {
		if nested, ok := rawJSONObject(normalizedRawJSON(obj["json_schema"])); ok {
			schema = normalizedRawJSON(nested["schema"])
		}
	}
	if len(schema) == 0 {
		return nil
	}
	out, err := json.Marshal(map[string]json.RawMessage{
		"type":   rawJSONString("json_schema"),
		"schema": schema,
	})
	if err != nil {
		return nil
	}
	return out
}

func normalizedRawJSON(raw json.RawMessage) json.RawMessage {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func rawJSONObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	return obj, true
}

func rawJSONString(value string) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
