package tokiame

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"

	"one-api/common"
	"one-api/common/requester"
	"one-api/providers/openai"
	"one-api/types"
)

type completionStreamHandler struct {
	usage *types.Usage
	model string
}

func (h *completionStreamHandler) handle(rawLine *[]byte, dataChan chan string, errChan chan error) {
	if !strings.HasPrefix(string(*rawLine), "data: ") {
		*rawLine = nil
		return
	}

	*rawLine = (*rawLine)[6:]
	if string(*rawLine) == "[DONE]" {
		errChan <- io.EOF
		*rawLine = requester.StreamClosed
		return
	}

	var response openai.OpenAIProviderCompletionResponse
	if err := common.Unmarshal(*rawLine, &response); err != nil {
		errChan <- common.ErrorToOpenAIError(err)
		return
	}

	if openAIError := openai.ErrorHandle(&response.OpenAIErrorResponse); openAIError != nil {
		errChan <- openAIError
		return
	}

	if len(response.Choices) == 0 {
		if response.Usage != nil {
			*h.usage = *response.Usage
		}
		*rawLine = nil
		return
	}

	if h.usage.TotalTokens == 0 {
		h.usage.TotalTokens = h.usage.PromptTokens
	}
	for _, choice := range response.Choices {
		h.usage.TextBuilder.WriteString(choice.Text)
	}
	dataChan <- string(*rawLine)
}

func hasJSONAudioResponse(request *types.AudioRequest) bool {
	return request.ResponseFormat == "" || request.ResponseFormat == "json" || request.ResponseFormat == "verbose_json"
}

func getAudioTextContent(text, format string) string {
	switch format {
	case "srt":
		return extractTextFromSRT(text)
	case "vtt":
		return extractTextFromVTT(text)
	default:
		return text
	}
}

func extractTextFromVTT(vttContent string) string {
	scanner := bufio.NewScanner(strings.NewReader(vttContent))
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d{3} --> \d{2}:\d{2}:\d{2}\.\d{3}`)
	var text []string
	isStart := true

	for scanner.Scan() {
		line := scanner.Text()
		if isStart && strings.HasPrefix(line, "WEBVTT") {
			isStart = false
			continue
		}
		if !re.MatchString(line) && !isNumber(line) && line != "" {
			text = append(text, line)
		}
	}

	return strings.Join(text, " ")
}

func extractTextFromSRT(srtContent string) string {
	scanner := bufio.NewScanner(strings.NewReader(srtContent))
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}`)
	var text []string
	isContent := false

	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			isContent = true
		} else if line == "" {
			isContent = false
		} else if isContent {
			text = append(text, line)
		}
	}

	return strings.Join(text, " ")
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
