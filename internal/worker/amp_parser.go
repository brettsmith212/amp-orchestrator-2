package worker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AmpLogEntry represents a single JSON log entry from amp's log file
type AmpLogEntry struct {
	Level     string       `json:"level"`
	Message   string       `json:"message"`
	Timestamp time.Time    `json:"timestamp"`
	Event     *ThreadEvent `json:"event,omitempty"`
}

// ThreadEvent represents amp's thread-state events
type ThreadEvent struct {
	Type   string  `json:"type"`
	Thread *Thread `json:"thread,omitempty"`
}

// Thread represents the thread data from amp
type Thread struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Messages []Message `json:"messages"`
}

// Message represents a message in amp's thread
type Message struct {
	Role    string    `json:"role"` // "user" or "assistant"
	Content []Content `json:"content"`
	Meta    *MessageMeta `json:"meta,omitempty"`
	State   *MessageState `json:"state,omitempty"`
}

// Content represents the content of a message
type Content struct {
	Type     string                 `json:"type"` // "text", "thinking", "tool_use", "tool_result", etc.
	Text     string                 `json:"text,omitempty"`
	Thinking string                 `json:"thinking,omitempty"`
	ID       string                 `json:"id,omitempty"`       // For tool_use
	Name     string                 `json:"name,omitempty"`     // For tool_use
	Input    map[string]interface{} `json:"input,omitempty"`    // For tool_use
	Run      map[string]interface{} `json:"run,omitempty"`      // For tool_result
	ToolUseID string                `json:"toolUseID,omitempty"` // For tool_result
}

// MessageMeta contains message metadata
type MessageMeta struct {
	SentAt int64 `json:"sentAt"`
}

// MessageState contains message state
type MessageState struct {
	Type       string `json:"type"`       // "streaming", "complete", etc.
	StopReason string `json:"stopReason,omitempty"` // "end_turn", "tool_use", etc.
}

// AmpLogParser parses amp's JSON log output and reconstructs the final conversation
type AmpLogParser struct {
	workerID        string
	onMessage       func(ThreadMessage)
	latestThread    *Thread
	lastThreadUpdate time.Time
	conversationProcessed bool
}

// NewAmpLogParser creates a new amp log parser
func NewAmpLogParser(workerID string, onMessage func(ThreadMessage)) *AmpLogParser {
	return &AmpLogParser{
		workerID:  workerID,
		onMessage: onMessage,
	}
}

// ParseLine processes a single line from amp's JSON log file
func (p *AmpLogParser) ParseLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	
	var logEntry AmpLogEntry
	if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
		// Skip malformed JSON lines
		return
	}
	
	// Only process thread-state events which contain the conversation
	if logEntry.Event != nil && logEntry.Event.Type == "thread-state" && logEntry.Event.Thread != nil {
		p.updateThreadState(logEntry.Event.Thread, logEntry.Timestamp)
	}
}

// updateThreadState stores the latest complete thread state
func (p *AmpLogParser) updateThreadState(thread *Thread, timestamp time.Time) {
	p.latestThread = thread
	p.lastThreadUpdate = timestamp
	// Reset processed flag when we get new thread data
	p.conversationProcessed = false
}

// ProcessFinalConversation processes the complete conversation when amp is done
func (p *AmpLogParser) ProcessFinalConversation() {
	if p.latestThread == nil || p.conversationProcessed {
		return
	}
	
	// Emit thread start
	if p.latestThread.Title != "" {
		p.emitMessage(MessageTypeSystem, fmt.Sprintf("Thread: %s", p.latestThread.Title), p.lastThreadUpdate, map[string]interface{}{
			"thread_id": p.latestThread.ID,
			"thread_title": p.latestThread.Title,
		})
	}
	
	// Process each message in the final conversation
	for _, message := range p.latestThread.Messages {
		p.processMessage(message, p.lastThreadUpdate)
	}
	
	p.conversationProcessed = true
}

// processMessage converts an amp message to our thread message format
func (p *AmpLogParser) processMessage(ampMsg Message, timestamp time.Time) {
	// Use the message's sent time if available
	msgTime := timestamp
	if ampMsg.Meta != nil && ampMsg.Meta.SentAt > 0 {
		msgTime = time.Unix(ampMsg.Meta.SentAt/1000, (ampMsg.Meta.SentAt%1000)*1000000)
	}
	
	switch ampMsg.Role {
	case "user":
		p.processUserMessage(ampMsg, msgTime)
	case "assistant":
		p.processAssistantMessage(ampMsg, msgTime)
	}
}

// processUserMessage handles user messages
func (p *AmpLogParser) processUserMessage(ampMsg Message, msgTime time.Time) {
	for _, content := range ampMsg.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			p.emitMessage(MessageTypeUser, strings.TrimSpace(content.Text), msgTime, nil)
		}
		// Skip tool_result content as it's system-level feedback
	}
}

// processAssistantMessage handles assistant messages
func (p *AmpLogParser) processAssistantMessage(ampMsg Message, msgTime time.Time) {
	// Look for thinking content first
	for _, content := range ampMsg.Content {
		if content.Type == "thinking" && strings.TrimSpace(content.Thinking) != "" {
			metadata := map[string]interface{}{
				"type": "thinking",
			}
			p.emitMessage(MessageTypeAssistant, strings.TrimSpace(content.Thinking), msgTime, metadata)
		}
	}
	
	// Then look for tool usage
	for _, content := range ampMsg.Content {
		if content.Type == "tool_use" && content.Name != "" {
			toolDescription := p.formatToolUse(content)
			metadata := map[string]interface{}{
				"type":      "tool_use",
				"tool_name": content.Name,
				"tool_id":   content.ID,
				"input":     content.Input,
			}
			p.emitMessage(MessageTypeTool, toolDescription, msgTime, metadata)
		}
	}
	
	// Finally, look for the main text response
	for _, content := range ampMsg.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			p.emitMessage(MessageTypeAssistant, strings.TrimSpace(content.Text), msgTime, nil)
		}
	}
}

// formatToolUse creates a human-readable description of tool usage
func (p *AmpLogParser) formatToolUse(content Content) string {
	switch content.Name {
	case "create_file":
		if path, ok := content.Input["path"].(string); ok {
			return fmt.Sprintf("Creating file: %s", path)
		}
		return "Creating file"
		
	case "edit_file":
		if path, ok := content.Input["path"].(string); ok {
			return fmt.Sprintf("Editing file: %s", path)
		}
		return "Editing file"
		
	case "read_file":
		if path, ok := content.Input["path"].(string); ok {
			return fmt.Sprintf("Reading file: %s", path)
		}
		return "Reading file"
		
	case "Bash":
		if cmd, ok := content.Input["cmd"].(string); ok {
			// Truncate very long commands
			if len(cmd) > 100 {
				cmd = cmd[:97] + "..."
			}
			return fmt.Sprintf("Running command: %s", cmd)
		}
		return "Running command"
		
	case "Grep":
		if pattern, ok := content.Input["pattern"].(string); ok {
			return fmt.Sprintf("Searching for: %s", pattern)
		}
		return "Searching files"
		
	case "glob":
		if pattern, ok := content.Input["filePattern"].(string); ok {
			return fmt.Sprintf("Finding files: %s", pattern)
		}
		return "Finding files"
		
	default:
		return fmt.Sprintf("Using tool: %s", content.Name)
	}
}

// emitMessage sends a thread message
func (p *AmpLogParser) emitMessage(msgType MessageType, content string, timestamp time.Time, metadata map[string]interface{}) {
	if p.onMessage != nil && strings.TrimSpace(content) != "" {
		message := ThreadMessage{
			ID:        uuid.New().String(),
			Type:      msgType,
			Content:   content,
			Timestamp: timestamp,
			Metadata:  metadata,
		}
		p.onMessage(message)
	}
}

// LogTailerWithParser wraps a log tailer with amp log parsing
type LogTailerWithParser struct {
	*LogTailer
	parser *AmpLogParser
}

// NewLogTailerWithParser creates a new log tailer that parses amp's JSON log output
func NewLogTailerWithParser(logFile, workerID string, onLogLine func(LogLine), onThreadMessage func(ThreadMessage)) *LogTailerWithParser {
	parser := NewAmpLogParser(workerID, onThreadMessage)
	
	// Create a callback that parses the log file for thread messages
	wrappedCallback := func(logLine LogLine) {
		// Call original log callback for stdout logs
		if onLogLine != nil {
			onLogLine(logLine)
		}
		
		// Parse the amp log line for thread messages (JSON format)
		parser.ParseLine(logLine.Content)
	}
	
	tailer := NewLogTailer(logFile, workerID, wrappedCallback)
	
	return &LogTailerWithParser{
		LogTailer: tailer,
		parser:    parser,
	}
}

// ProcessFinalConversation exposes the parser's ProcessFinalConversation method
func (lt *LogTailerWithParser) ProcessFinalConversation() {
	if lt.parser != nil {
		lt.parser.ProcessFinalConversation()
	}
}
