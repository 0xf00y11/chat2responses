package proxy

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestStreamConverter_ReasoningAndText(t *testing.T) {
	upstreamData := "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Thinking... \"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"step 2.\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"world!\"}}]}\n" +
		"data: [DONE]\n"

	upstream := io.NopCloser(strings.NewReader(upstreamData))
	var buf bytes.Buffer

	sc := NewStreamConverter("test-model", "test-resp-id")
	err := sc.Convert(upstream, &buf)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	expectedReasoning := "Thinking... step 2."
	if sc.CollectedReasoning() != expectedReasoning {
		t.Errorf("expected reasoning %q, got %q", expectedReasoning, sc.CollectedReasoning())
	}

	expectedText := "Hello world!"
	if sc.CollectedText() != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, sc.CollectedText())
	}
}
