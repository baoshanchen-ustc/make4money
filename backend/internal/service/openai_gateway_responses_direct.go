package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type directResponsesStreamToolState struct {
	OutputIndex int
	ItemID      string
	CallID      string
	Name        string
}

type directResponsesStreamState struct {
	ResponseID       string
	Model            string
	CreatedUnix      int64
	CreatedSent      bool
	MessageItemID    string
	MessageStarted   bool
	ReasoningItemID  string
	ReasoningStarted bool
	ToolStates       map[int]*directResponsesStreamToolState
	Accumulator      *apicompat.BufferedResponseAccumulator
	Usage            OpenAIUsage
}

func newDirectResponsesStreamState(model string) *directResponsesStreamState {
	return &directResponsesStreamState{
		Model:       model,
		CreatedUnix: time.Now().Unix(),
		ToolStates:  make(map[int]*directResponsesStreamToolState),
		Accumulator: apicompat.NewBufferedResponseAccumulator(),
	}
}

func (s *OpenAIGatewayService) forwardOpenAIResponsesViaChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(body, &responsesReq); err != nil {
		return nil, fmt.Errorf("parse responses request: %w", err)
	}
	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(&responsesReq)
	if err != nil {
		return nil, fmt.Errorf("convert responses request to chat completions: %w", err)
	}
	chatReq.Model = upstreamModel

	baseURL := account.GetOpenAIBaseURL()
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)

	upstreamBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completions request: %w", err)
	}
	setOpsUpstreamRequestBody(c, upstreamBody)

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+token)
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiAllowedHeaders[lowerKey] {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("user-agent", customUA)
	}
	if s.cfg != nil && s.cfg.Gateway.ForceCodexCLI {
		req.Header.Set("user-agent", codexCLIUserAgent)
	}
	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}
	if chatReq.Stream {
		req.Header.Set("accept", "text/event-stream")
	} else {
		req.Header.Set("accept", "application/json")
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"type":    "upstream_error",
				"message": "Upstream request failed",
			},
		})
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if upstreamMsg == "" {
			upstreamMsg = string(respBody)
		}
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, "")
		c.JSON(mapUpstreamStatusCode(resp.StatusCode), gin.H{
			"error": gin.H{
				"type":    "upstream_error",
				"message": upstreamMsg,
			},
		})
		return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
	}

	if chatReq.Stream {
		return s.handleDirectResponsesStream(resp, c, originalModel, billingModel, upstreamModel, startTime)
	}
	return s.handleDirectResponsesJSON(resp, c, originalModel, billingModel, upstreamModel, startTime)
}

func (s *OpenAIGatewayService) handleDirectResponsesJSON(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var chatResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, s.writeOpenAINonStreamingProtocolError(resp, c, "Failed to parse chat completions response")
	}

	responsesResp := apicompat.ChatCompletionsToResponsesResponse(&chatResp, originalModel)
	respBody, err := json.Marshal(responsesResp)
	if err != nil {
		return nil, err
	}

	usage := OpenAIUsage{
		InputTokens:              int(gjson.GetBytes(body, "usage.prompt_tokens").Int()),
		OutputTokens:             int(gjson.GetBytes(body, "usage.completion_tokens").Int()),
		CacheReadInputTokens:     int(gjson.GetBytes(body, "usage.prompt_tokens_details.cached_tokens").Int()),
		CacheCreationInputTokens: 0,
	}

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", respBody)

	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		Stream:          false,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) handleDirectResponsesStream(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	state := newDirectResponsesStreamState(originalModel)
	reader := bufio.NewReader(resp.Body)
	var firstTokenMs *int
	completed := false

	emit := func(evt apicompat.ResponsesStreamEvent) error {
		switch evt.Type {
		case "response.output_item.added", "response.output_text.delta", "response.reasoning_summary_text.delta", "response.function_call_arguments.delta":
			state.Accumulator.ProcessEvent(&evt)
		case "response.completed", "response.incomplete":
			completed = true
		}
		formatted, err := apicompat.ResponsesEventToSSE(evt)
		if err != nil {
			return err
		}
		_, err = io.WriteString(c.Writer, formatted)
		if err == nil {
			c.Writer.Flush()
		}
		return err
	}

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			trimmed := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(trimmed, "data: ") {
				payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data: "))
				if payload != "" && payload != "[DONE]" {
					if firstTokenMs == nil {
						ms := int(time.Since(startTime).Milliseconds())
						firstTokenMs = &ms
					}
					var chunk apicompat.ChatCompletionsChunk
					if json.Unmarshal([]byte(payload), &chunk) == nil {
						events := directResponsesEventsFromChatChunk(state, &chunk)
						for _, evt := range events {
							if writeErr := emit(evt); writeErr != nil {
								return buildDirectResponsesStreamResult(requestID, state, originalModel, billingModel, upstreamModel, resp, startTime, firstTokenMs), nil
							}
						}
					}
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				if !completed {
					for _, evt := range finalizeDirectResponsesEvents(state) {
						if writeErr := emit(evt); writeErr != nil {
							break
						}
					}
				}
				_, _ = io.WriteString(c.Writer, "data: [DONE]\n\n")
				c.Writer.Flush()
				return buildDirectResponsesStreamResult(requestID, state, originalModel, billingModel, upstreamModel, resp, startTime, firstTokenMs), nil
			}
			return nil, err
		}
	}
}

func buildDirectResponsesStreamResult(
	requestID string,
	state *directResponsesStreamState,
	originalModel string,
	billingModel string,
	upstreamModel string,
	resp *http.Response,
	startTime time.Time,
	firstTokenMs *int,
) *OpenAIForwardResult {
	return &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           state.Usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		Stream:          true,
		ResponseHeaders: resp.Header.Clone(),
		FirstTokenMs:    firstTokenMs,
		Duration:        time.Since(startTime),
	}
}

func directResponsesEventsFromChatChunk(state *directResponsesStreamState, chunk *apicompat.ChatCompletionsChunk) []apicompat.ResponsesStreamEvent {
	if chunk == nil {
		return nil
	}
	if chunk.ID != "" {
		state.ResponseID = chunk.ID
	}
	if chunk.Model != "" {
		state.Model = chunk.Model
	}
	if chunk.Usage != nil {
		state.Usage.InputTokens = chunk.Usage.PromptTokens
		state.Usage.OutputTokens = chunk.Usage.CompletionTokens
		if chunk.Usage.PromptTokensDetails != nil {
			state.Usage.CacheReadInputTokens = chunk.Usage.PromptTokensDetails.CachedTokens
		}
	}

	var events []apicompat.ResponsesStreamEvent
	if !state.CreatedSent {
		state.CreatedSent = true
		events = append(events, apicompat.ResponsesStreamEvent{
			Type: "response.created",
			Response: &apicompat.ResponsesResponse{
				ID:     state.responseID(),
				Object: "response",
				Model:  state.Model,
				Status: "in_progress",
			},
		})
	}

	for _, choice := range chunk.Choices {
		if delta := choice.Delta.ReasoningContent; delta != nil && *delta != "" {
			if !state.ReasoningStarted {
				state.ReasoningStarted = true
				state.ReasoningItemID = fmt.Sprintf("%s_reasoning", state.responseID())
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:        "response.output_item.added",
					OutputIndex: 1,
					Item: &apicompat.ResponsesOutput{
						Type:   "reasoning",
						ID:     state.ReasoningItemID,
						Status: "in_progress",
					},
				})
			}
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:         "response.reasoning_summary_text.delta",
				OutputIndex:  1,
				SummaryIndex: 0,
				ItemID:       state.ReasoningItemID,
				Delta:        *delta,
			})
		}

		if delta := choice.Delta.Content; delta != nil && *delta != "" {
			if !state.MessageStarted {
				state.MessageStarted = true
				state.MessageItemID = fmt.Sprintf("%s_message", state.responseID())
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:        "response.output_item.added",
					OutputIndex: 0,
					Item: &apicompat.ResponsesOutput{
						Type:   "message",
						ID:     state.MessageItemID,
						Role:   "assistant",
						Status: "in_progress",
					},
				})
			}
			events = append(events, apicompat.ResponsesStreamEvent{
				Type:         "response.output_text.delta",
				OutputIndex:  0,
				ContentIndex: 0,
				ItemID:       state.MessageItemID,
				Delta:        *delta,
			})
		}

		for idx, toolCall := range choice.Delta.ToolCalls {
			toolIndex := idx
			if toolCall.Index != nil {
				toolIndex = *toolCall.Index
			}
			toolState, exists := state.ToolStates[toolIndex]
			if !exists {
				callID := toolCall.ID
				if callID == "" {
					callID = fmt.Sprintf("%s_call_%d", state.responseID(), toolIndex)
				}
				toolState = &directResponsesStreamToolState{
					OutputIndex: 2 + toolIndex,
					ItemID:      fmt.Sprintf("%s_tool_%d", state.responseID(), toolIndex),
					CallID:      callID,
					Name:        toolCall.Function.Name,
				}
				state.ToolStates[toolIndex] = toolState
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:        "response.output_item.added",
					OutputIndex: toolState.OutputIndex,
					Item: &apicompat.ResponsesOutput{
						Type:   "function_call",
						ID:     toolState.ItemID,
						CallID: toolState.CallID,
						Name:   toolState.Name,
						Status: "in_progress",
					},
				})
			} else {
				if toolState.CallID == "" && toolCall.ID != "" {
					toolState.CallID = toolCall.ID
				}
				if toolState.Name == "" && toolCall.Function.Name != "" {
					toolState.Name = toolCall.Function.Name
				}
			}
			if toolCall.Function.Arguments != "" {
				events = append(events, apicompat.ResponsesStreamEvent{
					Type:        "response.function_call_arguments.delta",
					OutputIndex: toolState.OutputIndex,
					ItemID:      toolState.ItemID,
					CallID:      toolState.CallID,
					Name:        toolState.Name,
					Delta:       toolCall.Function.Arguments,
				})
			}
		}

		if choice.FinishReason != nil {
			status := "completed"
			if *choice.FinishReason == "length" {
				status = "incomplete"
			}
			events = append(events, buildDirectResponsesTerminalEvent(state, status))
		}
	}

	return events
}

func finalizeDirectResponsesEvents(state *directResponsesStreamState) []apicompat.ResponsesStreamEvent {
	return []apicompat.ResponsesStreamEvent{buildDirectResponsesTerminalEvent(state, "completed")}
}

func buildDirectResponsesTerminalEvent(state *directResponsesStreamState, status string) apicompat.ResponsesStreamEvent {
	response := &apicompat.ResponsesResponse{
		ID:     state.responseID(),
		Object: "response",
		Model:  state.Model,
		Status: status,
		Output: state.Accumulator.BuildOutput(),
	}
	if len(response.Output) == 0 {
		response.Output = []apicompat.ResponsesOutput{{
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []apicompat.ResponsesContentPart{{
				Type: "output_text",
				Text: "",
			}},
		}}
	}
	response.Usage = &apicompat.ResponsesUsage{
		InputTokens:  state.Usage.InputTokens,
		OutputTokens: state.Usage.OutputTokens,
		TotalTokens:  state.Usage.InputTokens + state.Usage.OutputTokens,
	}
	if state.Usage.CacheReadInputTokens > 0 {
		response.Usage.InputTokensDetails = &apicompat.ResponsesInputTokensDetails{
			CachedTokens: state.Usage.CacheReadInputTokens,
		}
	}
	if status == "incomplete" {
		response.IncompleteDetails = &apicompat.ResponsesIncompleteDetails{
			Reason: "max_output_tokens",
		}
	}
	return apicompat.ResponsesStreamEvent{
		Type:     "response." + status,
		Response: response,
	}
}

func (s *directResponsesStreamState) responseID() string {
	if s.ResponseID != "" {
		return s.ResponseID
	}
	s.ResponseID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	return s.ResponseID
}
