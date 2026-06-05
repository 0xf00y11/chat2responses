package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"chat2responses/internal/model"
)

type toolCallBuilder struct {
	Index            int
	ID               string
	Name             string
	Args             strings.Builder
	ThoughtSignature string
}

type StreamConverter struct {
	model              string
	respID             string
	collectedText      strings.Builder
	collectedReasoning strings.Builder
	collectedToolCalls []model.ChatToolCall
	reasoningStarted   bool
	textStarted        bool
}

func NewStreamConverter(model, respID string) *StreamConverter {
	return &StreamConverter{model: model, respID: respID}
}

func (sc *StreamConverter) CollectedText() string {
	return sc.collectedText.String()
}

func (sc *StreamConverter) CollectedToolCalls() []model.ChatToolCall {
	return sc.collectedToolCalls
}

func (sc *StreamConverter) Convert(upstream io.ReadCloser, w io.Writer) error {
	defer upstream.Close()

	scanner := bufio.NewScanner(upstream)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var currentModel string
	var usageData map[string]interface{}
	var tcBuilders []*toolCallBuilder

	msgID := model.MakeID()
	partIndex := 0

	sendEvent := func(evt interface{}) error {
		data, err := json.Marshal(evt)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
		return err
	}

	// Send initial response.created event
	sendEvent(map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":     sc.respID,
			"object": "response",
			"status": "in_progress",
		},
	})

	// Send output_item.added for the response message
	sendEvent(map[string]interface{}{
		"type":         "response.output_item.added",
		"output_index": 0,
		"item": map[string]interface{}{
			"id":      msgID,
			"type":    "message",
			"role":    "assistant",
			"status":  "in_progress",
			"content": []interface{}{},
		},
	})

	// beginReasoning sends a content_part.added for a reasoning block (once)
	beginReasoning := func() {
		if sc.reasoningStarted {
			return
		}
		sc.reasoningStarted = true
		sendEvent(map[string]interface{}{
			"type":         "response.content_part.added",
			"item_id":      msgID,
			"output_index": 0,
			"part_index":   partIndex,
			"part": map[string]interface{}{
				"type": "reasoning",
				"reasoning": map[string]interface{}{
					"content": "",
				},
			},
		})
	}

	// beginText sends a content_part.added for an output_text block (once)
	beginText := func() {
		if sc.textStarted {
			return
		}
		sc.textStarted = true
		// If reasoning was already started, increment part index
		if sc.reasoningStarted {
			partIndex = 1
		}
		sendEvent(map[string]interface{}{
			"type":         "response.content_part.added",
			"item_id":      msgID,
			"output_index": 0,
			"part_index":   partIndex,
			"part": map[string]interface{}{
				"type":        "output_text",
				"text":        "",
				"annotations": []interface{}{},
			},
		})
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		slog.Info("upstream stream chunk", "data", data)

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if m, ok := chunk["model"].(string); ok && currentModel == "" {
			currentModel = m
		}

		choices, ok := chunk["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}

		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}

		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		// Capture usage from final chunk
		if u, ok := chunk["usage"].(map[string]interface{}); ok {
			usageData = u
		}

		// Reasoning content — must start before any text content
		if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
			beginReasoning()
			sc.collectedReasoning.WriteString(reasoning)
			sendEvent(map[string]interface{}{
				"type":         "response.reasoning.delta",
				"item_id":      msgID,
				"output_index": 0,
				"delta": map[string]interface{}{
					"content": reasoning,
				},
			})
		}

		// Content delta
		if content, ok := delta["content"].(string); ok && content != "" {
			beginText()
			sc.collectedText.WriteString(content)
			sendEvent(map[string]interface{}{
				"type":         "response.output_text.delta",
				"item_id":      msgID,
				"output_index": 0,
				"delta":        content,
			})
		}

		// Tool calls delta
		if tcRaw, ok := delta["tool_calls"]; ok {
			tcList, ok := tcRaw.([]interface{})
			if !ok {
				continue
			}
			for _, tcItem := range tcList {
				tc, ok := tcItem.(map[string]interface{})
				if !ok {
					continue
				}

				index := 0
				if idx, ok := tc["index"].(float64); ok {
					index = int(idx)
				}

				var builder *toolCallBuilder
				for _, b := range tcBuilders {
					if b.Index == index {
						builder = b
						break
					}
				}

				if builder == nil {
					builder = &toolCallBuilder{Index: index}
					tcBuilders = append(tcBuilders, builder)

					if id, ok := tc["id"].(string); ok {
						builder.ID = id
					}
					if fn, ok := tc["function"].(map[string]interface{}); ok {
						if name, ok := fn["name"].(string); ok {
							builder.Name = name
						}
					}

					tcID := builder.ID
					if tcID == "" {
						tcID = model.MakeID("fc")
						builder.ID = tcID
					}

					sendEvent(map[string]interface{}{
						"type":         "response.output_item.added",
						"output_index": 1,
						"item": map[string]interface{}{
							"id":        builder.ID,
							"type":      "function_call",
							"status":    "in_progress",
							"name":      builder.Name,
							"call_id":   builder.ID,
							"arguments": "",
						},
					})
				}

				if fn, ok := tc["function"].(map[string]interface{}); ok {
					if args, ok := fn["arguments"].(string); ok {
						builder.Args.WriteString(args)
						sendEvent(map[string]interface{}{
							"type":         "response.function_call_arguments.delta",
							"item_id":      builder.ID,
							"output_index": 1,
							"delta":        args,
						})
					}
				}

				if id, ok := tc["id"].(string); ok && builder.ID == "" {
					builder.ID = id
				}
				if fn, ok := tc["function"].(map[string]interface{}); ok {
					if name, ok := fn["name"].(string); ok && builder.Name == "" {
						builder.Name = name
					}
				}
				if ts, ok := tc["thought_signature"].(string); ok && ts != "" {
					builder.ThoughtSignature = ts
				} else if ts, ok := tc["thoughtSignature"].(string); ok && ts != "" {
					builder.ThoughtSignature = ts
				} else if ec, ok := tc["extra_content"].(map[string]interface{}); ok {
					if google, ok := ec["google"].(map[string]interface{}); ok {
						if ts, ok := google["thought_signature"].(string); ok && ts != "" {
							builder.ThoughtSignature = ts
						} else if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
							builder.ThoughtSignature = ts
						}
					}
				} else if ec, ok := tc["extraContent"].(map[string]interface{}); ok {
					if google, ok := ec["google"].(map[string]interface{}); ok {
						if ts, ok := google["thought_signature"].(string); ok && ts != "" {
							builder.ThoughtSignature = ts
						} else if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
							builder.ThoughtSignature = ts
						}
					}
				} else if google, ok := tc["google"].(map[string]interface{}); ok {
					if ts, ok := google["thought_signature"].(string); ok && ts != "" {
						builder.ThoughtSignature = ts
					} else if ts, ok := google["thoughtSignature"].(string); ok && ts != "" {
						builder.ThoughtSignature = ts
					}
				} else if fn, ok := tc["function"].(map[string]interface{}); ok {
					if ts, ok := fn["thought_signature"].(string); ok && ts != "" {
						builder.ThoughtSignature = ts
					} else if ts, ok := fn["thoughtSignature"].(string); ok && ts != "" {
						builder.ThoughtSignature = ts
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner: %w", err)
	}

	// Build content blocks for the done events
	var contentBlocks []interface{}

	// Reasoning block done
	reasoningText := sc.collectedReasoning.String()
	if reasoningText != "" {
		block := map[string]interface{}{
			"type": "reasoning",
			"reasoning": map[string]interface{}{
				"content": reasoningText,
			},
		}
		contentBlocks = append(contentBlocks, block)
		sendEvent(map[string]interface{}{
			"type":         "response.content_part.done",
			"item_id":      msgID,
			"output_index": 0,
			"part_index":   0,
			"part":         block,
		})
	}

	// Output text block done
	collectedText := sc.collectedText.String()
	if collectedText != "" {
		block := map[string]interface{}{
			"type":        "output_text",
			"text":        collectedText,
			"annotations": []interface{}{},
		}
		contentBlocks = append(contentBlocks, block)
		textPartIndex := 0
		if sc.reasoningStarted {
			textPartIndex = 1
		}
		sendEvent(map[string]interface{}{
			"type":         "response.content_part.done",
			"item_id":      msgID,
			"output_index": 0,
			"part_index":   textPartIndex,
			"part":         block,
		})
	}

	// output_item.done for message (only if there were content blocks)
	if len(contentBlocks) > 0 {
		sendEvent(map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item": map[string]interface{}{
				"id":      msgID,
				"type":    "message",
				"role":    "assistant",
				"status":  "completed",
				"content": contentBlocks,
			},
		})
	}

	// Function call done events
	for _, builder := range tcBuilders {
		args := builder.Args.String()
		sendEvent(map[string]interface{}{
			"type":         "response.function_call_arguments.done",
			"item_id":      builder.ID,
			"output_index": 1,
			"arguments":    args,
		})
		itemMap := map[string]interface{}{
			"id":        builder.ID,
			"type":      "function_call",
			"call_id":   builder.ID,
			"name":      builder.Name,
			"arguments": args,
			"status":    "completed",
		}
		if builder.ThoughtSignature != "" {
			itemMap["thought_signature"] = builder.ThoughtSignature
		}
		sendEvent(map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": 1,
			"item":         itemMap,
		})

		sc.collectedToolCalls = append(sc.collectedToolCalls, model.ChatToolCall{
			ID:   builder.ID,
			Type: "function",
			Function: model.ChatFunction{
				Name:      builder.Name,
				Arguments: args,
			},
			ThoughtSignature: builder.ThoughtSignature,
		})
	}

	// Build output items for response.completed
	var outputItems []interface{}
	if len(contentBlocks) > 0 {
		outputItems = append(outputItems, map[string]interface{}{
			"id":      msgID,
			"type":    "message",
			"role":    "assistant",
			"status":  "completed",
			"content": contentBlocks,
		})
	}
	for _, builder := range tcBuilders {
		itemMap := map[string]interface{}{
			"id":        builder.ID,
			"type":      "function_call",
			"call_id":   builder.ID,
			"name":      builder.Name,
			"arguments": builder.Args.String(),
			"status":    "completed",
		}
		if builder.ThoughtSignature != "" {
			itemMap["thought_signature"] = builder.ThoughtSignature
		}
		outputItems = append(outputItems, itemMap)
	}

	// response.completed
	sendEvent(map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":     sc.respID,
			"object": "response",
			"status": "completed",
			"model":  currentModel,
			"output": outputItems,
			"usage":  buildUsage(usageData),
		},
	})

	fmt.Fprintf(w, "data: [DONE]\n\n")
	return nil
}

func buildUsage(u map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"input_tokens":  0,
		"output_tokens": 0,
		"total_tokens":  0,
	}
	if u == nil {
		return result
	}
	if v, ok := u["prompt_tokens"]; ok {
		result["input_tokens"] = v
	}
	if v, ok := u["completion_tokens"]; ok {
		result["output_tokens"] = v
	}
	if v, ok := u["total_tokens"]; ok {
		result["total_tokens"] = v
	}
	return result
}
