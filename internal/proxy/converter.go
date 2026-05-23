package proxy

import (
	"encoding/json"
	"fmt"
	"strings"

	"chat2responses/internal/model"
)

// InputToMessages converts a Responses API input payload into Chat Completions messages.
// It does NOT handle instructions (handled by the session-aware server layer).
func InputToMessages(body *model.ResponsesRequest) []model.ChatMessage {
	var messages []model.ChatMessage

	if len(body.Input) == 0 {
		return messages
	}

	var inputStr string
	if err := json.Unmarshal(body.Input, &inputStr); err == nil {
		messages = append(messages, model.ChatMessage{Role: "user", Content: inputStr})
		return messages
	}

	var inputItems []json.RawMessage
	if err := json.Unmarshal(body.Input, &inputItems); err != nil {
		messages = append(messages, model.ChatMessage{Role: "user", Content: string(body.Input)})
		return messages
	}

	// Process input array items
	var pending []map[string]interface{}
	flushToolCalls := func() {
		if len(pending) == 0 {
			return
		}
		var calls []model.ChatToolCall
		for _, tc := range pending {
			cid, _ := tc["call_id"].(string)
			if cid == "" {
				cid, _ = tc["id"].(string)
			}
			name, _ := tc["name"].(string)
			if name == "" {
				name, _ = tc["function"].(string)
			}
			args, _ := tc["arguments"].(string)
			if args == "" {
				if a, ok := tc["arguments"].(map[string]interface{}); ok {
					b, _ := json.Marshal(a)
					args = string(b)
				}
			}
			if cid == "" || name == "" {
				continue
			}
			calls = append(calls, model.ChatToolCall{
				ID:   cid,
				Type: "function",
				Function: model.ChatFunction{Name: name, Arguments: args},
			})
		}
		messages = append(messages, model.ChatMessage{
			Role:      "assistant",
			Content:   nil,
			ToolCalls: calls,
		})
		pending = nil
	}

	for _, raw := range inputItems {
		var item map[string]interface{}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		typ, _ := item["type"].(string)
		switch typ {
		case "function_call":
			pending = append(pending, item)
		case "function_call_output":
			flushToolCalls()
			cid, _ := item["call_id"].(string)
			output, _ := item["output"].(string)
			messages = append(messages, model.ChatMessage{
				Role:       "tool",
				Content:    output,
				ToolCallID: cid,
			})
		default:
			flushToolCalls()
			role, _ := item["role"].(string)
			if role == "developer" {
				role = "system"
			}
			if role == "" {
				role = "user"
			}
			messages = append(messages, model.ChatMessage{
				Role:    role,
				Content: extractContent(item),
			})
		}
	}
	flushToolCalls()

	return messages
}

func extractContent(item map[string]interface{}) interface{} {
	raw, ok := item["content"]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, p := range v {
			if m, ok := p.(map[string]interface{}); ok {
				typ, _ := m["type"].(string)
				switch typ {
				case "input_text":
					if txt, ok := m["text"].(string); ok {
						parts = append(parts, txt)
					}
				case "input_image":
					parts = append(parts, "[image]")
				default:
					if txt, ok := m["text"].(string); ok {
						parts = append(parts, txt)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", raw)
	}
}

func ChatToResponses(chat *model.ChatResponse, defaultModel, respID string) *model.ResponsesResponse {
	resp := &model.ResponsesResponse{
		ID:      respID,
		Object:  "response",
		Created: chat.Created,
		Status:  "completed",
		Model:   chat.Model,
	}
	if resp.Model == "" {
		resp.Model = defaultModel
	}

	if len(chat.Choices) > 0 {
		msg := chat.Choices[0].Message
		var items []model.ResponseOutputItem

		if txt, ok := msg.Content.(string); ok && txt != "" {
			items = append(items, model.ResponseOutputItem{
				ID:     model.MakeID(),
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []model.ContentBlock{{
					Type:        "output_text",
					Text:        txt,
					Annotations: []interface{}{},
				}},
			})
		}

		for _, tc := range msg.ToolCalls {
			if tc.ID == "" {
				continue
			}
			items = append(items, model.ResponseOutputItem{
				ID:        tc.ID,
				Type:      "function_call",
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Status:    "completed",
			})
		}

		resp.Output = items
		resp.Usage = chat.Usage
	}

	return resp
}
