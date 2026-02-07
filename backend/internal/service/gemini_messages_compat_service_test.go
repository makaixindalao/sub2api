package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestConvertClaudeToolsToGeminiTools_CustomType 测试custom类型工具转换
func TestConvertClaudeToolsToGeminiTools_CustomType(t *testing.T) {
	tests := []struct {
		name        string
		tools       any
		expectedLen int
		description string
	}{
		{
			name: "Standard tools",
			tools: []any{
				map[string]any{
					"name":         "get_weather",
					"description":  "Get weather info",
					"input_schema": map[string]any{"type": "object"},
				},
			},
			expectedLen: 1,
			description: "标准工具格式应该正常转换",
		},
		{
			name: "Custom type tool (MCP format)",
			tools: []any{
				map[string]any{
					"type": "custom",
					"name": "mcp_tool",
					"custom": map[string]any{
						"description":  "MCP tool description",
						"input_schema": map[string]any{"type": "object"},
					},
				},
			},
			expectedLen: 1,
			description: "Custom类型工具应该从custom字段读取",
		},
		{
			name: "Mixed standard and custom tools",
			tools: []any{
				map[string]any{
					"name":         "standard_tool",
					"description":  "Standard",
					"input_schema": map[string]any{"type": "object"},
				},
				map[string]any{
					"type": "custom",
					"name": "custom_tool",
					"custom": map[string]any{
						"description":  "Custom",
						"input_schema": map[string]any{"type": "object"},
					},
				},
			},
			expectedLen: 1,
			description: "混合工具应该都能正确转换",
		},
		{
			name: "Custom tool without custom field",
			tools: []any{
				map[string]any{
					"type": "custom",
					"name": "invalid_custom",
					// 缺少 custom 字段
				},
			},
			expectedLen: 0, // 应该被跳过
			description: "缺少custom字段的custom工具应该被跳过",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertClaudeToolsToGeminiTools(tt.tools)

			if tt.expectedLen == 0 {
				if result != nil {
					t.Errorf("%s: expected nil result, got %v", tt.description, result)
				}
				return
			}

			if result == nil {
				t.Fatalf("%s: expected non-nil result", tt.description)
			}

			if len(result) != 1 {
				t.Errorf("%s: expected 1 tool declaration, got %d", tt.description, len(result))
				return
			}

			toolDecl, ok := result[0].(map[string]any)
			if !ok {
				t.Fatalf("%s: result[0] is not map[string]any", tt.description)
			}

			funcDecls, ok := toolDecl["functionDeclarations"].([]any)
			if !ok {
				t.Fatalf("%s: functionDeclarations is not []any", tt.description)
			}

			toolsArr, _ := tt.tools.([]any)
			expectedFuncCount := 0
			for _, tool := range toolsArr {
				toolMap, _ := tool.(map[string]any)
				if toolMap["name"] != "" {
					// 检查是否为有效的custom工具
					if toolMap["type"] == "custom" {
						if toolMap["custom"] != nil {
							expectedFuncCount++
						}
					} else {
						expectedFuncCount++
					}
				}
			}

			if len(funcDecls) != expectedFuncCount {
				t.Errorf("%s: expected %d function declarations, got %d",
					tt.description, expectedFuncCount, len(funcDecls))
			}
		})
	}
}

func TestConvertClaudeMessagesToGeminiGenerateContent_AddsThoughtSignatureForToolUse(t *testing.T) {
	claudeReq := map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 10,
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hi"},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "ok"},
					map[string]any{
						"type":  "tool_use",
						"id":    "toolu_123",
						"name":  "default_api:write_file",
						"input": map[string]any{"path": "a.txt", "content": "x"},
						// no signature on purpose
					},
				},
			},
		},
		"tools": []any{
			map[string]any{
				"name":        "default_api:write_file",
				"description": "write file",
				"input_schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"path": map[string]any{"type": "string"}},
				},
			},
		},
	}
	b, _ := json.Marshal(claudeReq)

	out, err := convertClaudeMessagesToGeminiGenerateContent(b)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "\"functionCall\"") {
		t.Fatalf("expected functionCall in output, got: %s", s)
	}
	if !strings.Contains(s, "\"thoughtSignature\":\""+geminiDummyThoughtSignature+"\"") {
		t.Fatalf("expected injected thoughtSignature %q, got: %s", geminiDummyThoughtSignature, s)
	}
}

func TestEnsureGeminiFunctionCallThoughtSignatures_InsertsWhenMissing(t *testing.T) {
	geminiReq := map[string]any{
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{
						"functionCall": map[string]any{
							"name": "default_api:write_file",
							"args": map[string]any{"path": "a.txt"},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(geminiReq)
	out := ensureGeminiFunctionCallThoughtSignatures(b)
	s := string(out)
	if !strings.Contains(s, "\"thoughtSignature\":\""+geminiDummyThoughtSignature+"\"") {
		t.Fatalf("expected injected thoughtSignature %q, got: %s", geminiDummyThoughtSignature, s)
	}
}

func TestParseGeminiRateLimitResetTime_QuotaResetDelayFallback(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Rate limit exceeded","details":[{"metadata":{"quotaResetDelay":"45s"}}]}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	delta := *ts - start
	if delta < 43 || delta > 47 {
		t.Fatalf("expected reset delta around 45s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

// TestParseGeminiRateLimitResetTime_ParsesMillisecondsDuration 测试毫秒级 duration 能被解析并向上取整为秒级重置时间
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_ParsesMillisecondsDuration(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Rate limit exceeded","details":[{"metadata":{"quotaResetDelay":"373.801628ms"}}]}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	// ceil(0.373...) = 1s，允许一定误差。
	delta := *ts - start
	if delta < 0 || delta > 2 {
		t.Fatalf("expected reset delta around 1s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

// TestParseGeminiRateLimitResetTime_RetryInfoRetryDelay 测试 RetryInfo.retryDelay 解析（优先级最高）
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_RetryInfoRetryDelay(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Rate limit exceeded","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"0.847655010s"}]}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	// ceil(0.847...) = 1s，考虑到秒级时间戳与执行耗时，允许一定误差。
	delta := *ts - start
	if delta < 0 || delta > 2 {
		t.Fatalf("expected reset delta around 1s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

// TestParseGeminiRateLimitResetTime_PleaseRetryInMessage 测试兼容 "Please retry in Xs" 形式
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_PleaseRetryInMessage(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Please retry in 30s"}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	delta := *ts - start
	if delta < 28 || delta > 32 {
		t.Fatalf("expected reset delta around 30s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

// TestParseGeminiRateLimitResetTime_AfterSecondsInMessage 测试从 error.message 解析 “after Xs”
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_AfterSecondsInMessage(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Your quota will reset after 60s."}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	delta := *ts - start
	if delta < 58 || delta > 62 {
		t.Fatalf("expected reset delta around 60s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

// TestParseGeminiRateLimitResetTime_WeirdDetailsElements 测试 details 包含非对象元素时仍能解析
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_WeirdDetailsElements(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Rate limit exceeded","details":["oops",{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"2s"}]}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	delta := *ts - start
	if delta < 1 || delta > 3 {
		t.Fatalf("expected reset delta around 2s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

// TestParseGeminiRateLimitResetTime_PriorityOrder 测试解析优先级：RetryInfo > ErrorInfo.metadata.quotaResetDelay
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_PriorityOrder(t *testing.T) {
	start := time.Now().Unix()
	body := []byte(`{"error":{"message":"Rate limit exceeded","details":[` +
		`{"@type":"type.googleapis.com/google.rpc.ErrorInfo","metadata":{"quotaResetDelay":"45s"}},` +
		`{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"30s"}` +
		`]}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	// 若优先级正确，应取 30s 而不是 45s。
	delta := *ts - start
	if delta < 28 || delta > 32 {
		t.Fatalf("expected reset delta around 30s, got %ds (resetAt=%d, start=%d)", delta, *ts, start)
	}
}

func TestParseGeminiRateLimitResetTime_BackwardCompatible(t *testing.T) {
	// Ensure the old function still works
	body := []byte(`{"error":{"message":"Rate limit","details":[{"metadata":{"quotaResetDelay":"60s"}}]}}`)
	ts := ParseGeminiRateLimitResetTime(body)
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	// Should be approximately 60 seconds from now
	expected := time.Now().Unix() + 60
	diff := *ts - expected
	if diff < -5 || diff > 5 {
		t.Errorf("expected timestamp around %d, got %d", expected, *ts)
	}
}

// TestParseGeminiRateLimitResetTime_ZeroOrNegativeDurationReturnsNil 测试 0 或负数 duration 不应返回重置时间
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_ZeroOrNegativeDurationReturnsNil(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"error":{"message":"Rate limit exceeded","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"0s"}]}}`),
		[]byte(`{"error":{"message":"Rate limit exceeded","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"-1s"}]}}`),
	}
	for i, body := range bodies {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ts := ParseGeminiRateLimitResetTime(body)
			if ts != nil {
				t.Fatalf("expected nil timestamp, got %v", *ts)
			}
		})
	}
}

// TestParseGeminiRateLimitResetTime_ReturnsNilWhenNoMatch 测试无法解析重试延迟的情况
// (作者：mkx, 日期：2026-02-05)
func TestParseGeminiRateLimitResetTime_ReturnsNilWhenNoMatch(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wantNil bool
	}{
		{
			name:    "empty body",
			body:    []byte(""),
			wantNil: true,
		},
		{
			name:    "invalid json",
			body:    []byte("not json"),
			wantNil: true,
		},
		{
			name:    "no error field",
			body:    []byte(`{"status": 429}`),
			wantNil: true,
		},
		{
			name:    "generic error message",
			body:    []byte(`{"error":{"message":"Resource exhausted"}}`),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantNil {
				if ts := ParseGeminiRateLimitResetTime(tt.body); ts != nil {
					t.Errorf("expected nil timestamp, got %v", *ts)
				}
			}
		})
	}
}
