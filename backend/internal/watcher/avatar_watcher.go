package watcher

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"multi-avatar-chat/internal/assistant"
	"multi-avatar-chat/internal/db"
	"multi-avatar-chat/internal/logic"
	"multi-avatar-chat/internal/models"
)

const (
	// minRandomInterval is the minimum interval for random polling (5 seconds)
	minRandomInterval = 5 * time.Second
	// maxRandomInterval is the maximum interval for random polling (20 seconds)
	maxRandomInterval = 20 * time.Second
)

// getRandomInterval returns a random duration between 5 and 20 seconds
func getRandomInterval() time.Duration {
	rangeNanos := int64(maxRandomInterval - minRandomInterval)
	randomNanos := rand.Int63n(rangeNanos)
	return minRandomInterval + time.Duration(randomNanos)
}

// BroadcastFunc is a callback function for broadcasting messages
type BroadcastFunc func(conversationID int64, msg *models.Message, senderName string)

// AvatarWatcher monitors conversation for a specific avatar
type AvatarWatcher struct {
	conversationID    int64
	conversationTitle string
	participantNames  []string
	avatar            models.Avatar
	db                *db.DB
	assistant         *assistant.Client
	interval          time.Duration
	useRandomInterval bool
	lastMessageID     int64
	broadcastFn       BroadcastFunc
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	// Fields for tracking active run (protected by mu)
	mu            sync.RWMutex
	currentRunID  string
	currentThreadID string
}

// NewAvatarWatcher creates a new AvatarWatcher
// If interval is 0, uses random intervals (5-20 seconds) for more natural responses
// Otherwise, uses the specified fixed interval (useful for testing)
func NewAvatarWatcher(
	parentCtx context.Context,
	conversationID int64,
	avatar models.Avatar,
	database *db.DB,
	assistantClient *assistant.Client,
	interval time.Duration,
	broadcastFn BroadcastFunc,
) *AvatarWatcher {
	ctx, cancel := context.WithCancel(parentCtx)

	// If interval is 0, use random interval mode
	useRandom := interval == 0

	return &AvatarWatcher{
		conversationID:    conversationID,
		avatar:            avatar,
		db:                database,
		assistant:         assistantClient,
		interval:          interval,
		useRandomInterval: useRandom,
		broadcastFn:       broadcastFn,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// SetConversationContext sets the conversation title and participant names
func (w *AvatarWatcher) SetConversationContext(title string, participantNames []string) {
	w.conversationTitle = title
	w.participantNames = participantNames
}

// Start begins the monitoring loop
func (w *AvatarWatcher) Start() {
	w.wg.Add(1)
	go w.run()
}

// Stop stops the monitoring loop and waits for it to finish
func (w *AvatarWatcher) Stop() {
	w.cancel()
	w.wg.Wait()
}

// Interrupt cancels any active LLM run and stops the watcher
func (w *AvatarWatcher) Interrupt() {
	log.Printf("[AvatarWatcher] Interrupt called conversation_id=%d avatar_id=%d avatar_name=%s",
		w.conversationID, w.avatar.ID, w.avatar.Name)

	// Cancel context to stop the watcher loop
	w.cancel()

	// Cancel any active run
	w.mu.RLock()
	runID := w.currentRunID
	threadID := w.currentThreadID
	w.mu.RUnlock()

	if runID != "" && threadID != "" && w.assistant != nil {
		log.Printf("[AvatarWatcher] Cancelling active run conversation_id=%d avatar_id=%d run_id=%s thread_id=%s",
			w.conversationID, w.avatar.ID, runID, threadID)
		if err := w.assistant.CancelRun(threadID, runID); err != nil {
			log.Printf("[AvatarWatcher] Failed to cancel run conversation_id=%d avatar_id=%d run_id=%s err=%v",
				w.conversationID, w.avatar.ID, runID, err)
		} else {
			log.Printf("[AvatarWatcher] Run cancelled successfully conversation_id=%d avatar_id=%d run_id=%s",
				w.conversationID, w.avatar.ID, runID)
		}
	}

	// Wait for watcher to finish
	w.wg.Wait()
}

func (w *AvatarWatcher) run() {
	defer w.wg.Done()

	log.Printf("[AvatarWatcher] Started conversation_id=%d avatar_id=%d avatar_name=%s useRandomInterval=%v interval=%v",
		w.conversationID, w.avatar.ID, w.avatar.Name, w.useRandomInterval, w.interval)

	// Initialize lastMessageID with the current latest message
	if err := w.initializeLastMessageID(); err != nil {
		log.Printf("[AvatarWatcher] Failed to initialize lastMessageID conversation_id=%d avatar_id=%d err=%v",
			w.conversationID, w.avatar.ID, err)
	}

	// Use random interval in production, fixed interval for testing
	if w.useRandomInterval {
		w.runWithRandomInterval()
	} else {
		w.runWithFixedInterval()
	}
}

// runWithFixedInterval runs the watcher with a fixed interval (for testing)
func (w *AvatarWatcher) runWithFixedInterval() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[AvatarWatcher] Stopped conversation_id=%d avatar_id=%d",
				w.conversationID, w.avatar.ID)
			return
		case <-ticker.C:
			if err := w.checkAndRespond(); err != nil {
				log.Printf("[AvatarWatcher] Error during check conversation_id=%d avatar_id=%d err=%v",
					w.conversationID, w.avatar.ID, err)
			}
		}
	}
}

// runWithRandomInterval runs the watcher with random intervals (5-20 seconds)
func (w *AvatarWatcher) runWithRandomInterval() {
	for {
		interval := getRandomInterval()
		log.Printf("[AvatarWatcher] Next check in %v conversation_id=%d avatar_id=%d",
			interval, w.conversationID, w.avatar.ID)

		select {
		case <-w.ctx.Done():
			log.Printf("[AvatarWatcher] Stopped conversation_id=%d avatar_id=%d",
				w.conversationID, w.avatar.ID)
			return
		case <-time.After(interval):
			if err := w.checkAndRespond(); err != nil {
				log.Printf("[AvatarWatcher] Error during check conversation_id=%d avatar_id=%d err=%v",
					w.conversationID, w.avatar.ID, err)
			}
		}
	}
}

// initializeLastMessageID sets lastMessageID to the current latest message
func (w *AvatarWatcher) initializeLastMessageID() error {
	messages, err := w.db.GetMessages(w.conversationID)
	if err != nil {
		return err
	}

	if len(messages) > 0 {
		w.lastMessageID = messages[len(messages)-1].ID
	}

	log.Printf("[AvatarWatcher] Initialized lastMessageID=%d conversation_id=%d avatar_id=%d",
		w.lastMessageID, w.conversationID, w.avatar.ID)
	return nil
}

// checkAndRespond checks for new messages and responds if appropriate
func (w *AvatarWatcher) checkAndRespond() error {
	// Get new messages since last check
	messages, err := w.db.GetMessagesAfter(w.conversationID, w.lastMessageID)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		return nil
	}

	log.Printf("[AvatarWatcher] Found %d new messages conversation_id=%d avatar_id=%d",
		len(messages), w.conversationID, w.avatar.ID)

	// Process each message
	for _, msg := range messages {
		// Update lastMessageID
		if msg.ID > w.lastMessageID {
			w.lastMessageID = msg.ID
		}

		// Skip own messages
		if msg.SenderType == models.SenderTypeAvatar && msg.SenderID != nil && *msg.SenderID == w.avatar.ID {
			continue
		}

		// Check if should respond
		shouldRespond, err := w.shouldRespond(&msg)
		if err != nil {
			log.Printf("[AvatarWatcher] Error checking shouldRespond message_id=%d err=%v", msg.ID, err)
			continue
		}

		if shouldRespond {
			if err := w.generateResponse(&msg); err != nil {
				log.Printf("[AvatarWatcher] Error generating response message_id=%d err=%v", msg.ID, err)
			}
		}
	}

	return nil
}

// shouldRespond determines if the avatar should respond to the message
func (w *AvatarWatcher) shouldRespond(message *models.Message) (bool, error) {
	// Check for direct mention
	mentionedNames := logic.ParseMentions(message.Content)
	for _, name := range mentionedNames {
		if strings.EqualFold(name, w.avatar.Name) {
			log.Printf("[AvatarWatcher] Mentioned in message message_id=%d avatar_name=%s",
				message.ID, w.avatar.Name)
			return true, nil
		}
	}

	// If no assistant configured, skip LLM judgment
	if w.assistant == nil || w.avatar.OpenAIAssistantID == "" {
		return false, nil
	}

	// LLM-based judgment
	return w.shouldRespondLLM(message)
}

// shouldRespondLLM uses LLM to determine if avatar should respond
func (w *AvatarWatcher) shouldRespondLLM(message *models.Message) (bool, error) {
	prompt := w.buildJudgmentPrompt(message.Content)

	// Use a simple completion request for judgment
	response, err := w.assistant.SimpleCompletion(prompt)
	if err != nil {
		log.Printf("[AvatarWatcher] LLM judgment failed message_id=%d err=%v", message.ID, err)
		return false, err
	}

	answer := strings.TrimSpace(strings.ToLower(response))
	shouldRespond := answer == "yes"

	log.Printf("[AvatarWatcher] LLM judgment message_id=%d avatar_name=%s answer=%q should_respond=%v",
		message.ID, w.avatar.Name, answer, shouldRespond)

	return shouldRespond, nil
}

// buildJudgmentPrompt creates the prompt for response judgment
func (w *AvatarWatcher) buildJudgmentPrompt(messageContent string) string {
	// Build participants section
	participantsSection := ""
	if len(w.participantNames) > 0 {
		participantsSection = "\n【Participants】\n"
		for _, name := range w.participantNames {
			if name == "ユーザ" || name == "User" {
				participantsSection += "- " + name + "\n"
			} else {
				participantsSection += "- (Avatar) " + name + "\n"
			}
		}
	}

	// Build topic section
	topicSection := ""
	if w.conversationTitle != "" {
		topicSection = "\n【Topic】\n" + w.conversationTitle + "\n"
	}

	return `You are "` + w.avatar.Name + `" character.
` + topicSection + participantsSection + `
【Your Settings】
` + w.avatar.Prompt + `

【Task】
Read the following message and determine whether you should respond to it.

Criteria:
- Is the content related to your specialty or role?
- Are you being directly addressed?
- Can you provide useful information?
- Should you speak based on the conversation flow?

【Message】
` + messageContent + `

【Answer】
Answer only "yes" if you should respond, or "no" if not.`
}

// generateResponse generates and saves a response from the avatar
func (w *AvatarWatcher) generateResponse(message *models.Message) error {
	log.Printf("[AvatarWatcher] Generating response conversation_id=%d avatar_id=%d avatar_name=%s message_id=%d",
		w.conversationID, w.avatar.ID, w.avatar.Name, message.ID)

	// Get avatar-specific thread ID
	threadID, err := w.db.GetAvatarThreadID(w.conversationID, w.avatar.ID)
	if err != nil {
		log.Printf("[AvatarWatcher] Failed to get avatar thread ID conversation_id=%d avatar_id=%d err=%v", w.conversationID, w.avatar.ID, err)
		return err
	}

	if threadID == "" || w.avatar.OpenAIAssistantID == "" {
		log.Printf("[AvatarWatcher] Cannot generate response: missing thread_id or assistant_id conversation_id=%d avatar_id=%d thread_id=%q assistant_id=%q",
			w.conversationID, w.avatar.ID, threadID, w.avatar.OpenAIAssistantID)
		return nil
	}

	// Wait for any active runs to complete before creating a new run
	if err := w.assistant.WaitForActiveRunsToComplete(threadID, 30*time.Second); err != nil {
		log.Printf("[AvatarWatcher] Timeout waiting for active runs thread_id=%s avatar_name=%s err=%v", threadID, w.avatar.Name, err)
		return err
	}

	// Build additional context from conversation history
	additionalContext := w.buildConversationContext()

	log.Printf("[AvatarWatcher] LLM Input thread_id=%s avatar_name=%s conversation_context_length=%d assistant_id=%s",
		threadID, w.avatar.Name, len(additionalContext), w.avatar.OpenAIAssistantID)
	if additionalContext != "" {
		log.Printf("[AvatarWatcher] LLM Input conversation_context=%q", additionalContext)
	}

	// Create a run with context
	var run *assistant.Run
	if additionalContext != "" {
		run, err = w.assistant.CreateRunWithContext(threadID, w.avatar.OpenAIAssistantID, additionalContext)
	} else {
		run, err = w.assistant.CreateRun(threadID, w.avatar.OpenAIAssistantID)
	}
	if err != nil {
		return err
	}

	// Track the active run
	w.mu.Lock()
	w.currentRunID = run.ID
	w.currentThreadID = threadID
	w.mu.Unlock()

	// Wait for completion (30 second timeout)
	_, err = w.assistant.WaitForRun(threadID, run.ID, 30*time.Second)
	
	// Clear the active run
	w.mu.Lock()
	w.currentRunID = ""
	w.currentThreadID = ""
	w.mu.Unlock()
	
	if err != nil {
		return err
	}

	// Get response
	responseContent, err := w.assistant.GetLatestAssistantMessage(threadID)
	if err != nil {
		return err
	}

	// Save to database
	avatarID := w.avatar.ID
	savedMsg, err := w.db.CreateMessage(w.conversationID, models.SenderTypeAvatar, &avatarID, responseContent)
	if err != nil {
		return err
	}

	// Update lastMessageID to include our own message
	if savedMsg.ID > w.lastMessageID {
		w.lastMessageID = savedMsg.ID
	}

	log.Printf("[AvatarWatcher] Response generated conversation_id=%d avatar_id=%d avatar_name=%s response_message_id=%d",
		w.conversationID, w.avatar.ID, w.avatar.Name, savedMsg.ID)

	// Broadcast the message via SSE
	if w.broadcastFn != nil {
		w.broadcastFn(w.conversationID, savedMsg, w.avatar.Name)
		log.Printf("[AvatarWatcher] Message broadcasted via SSE conversation_id=%d message_id=%d",
			w.conversationID, savedMsg.ID)
	}

	// Send the avatar's message to other avatars' threads
	if err := w.broadcastMessageToOtherAvatars(responseContent); err != nil {
		log.Printf("[AvatarWatcher] Warning: failed to broadcast message to other avatars conversation_id=%d avatar_id=%d err=%v",
			w.conversationID, w.avatar.ID, err)
		// Continue - message is saved and broadcasted via SSE
	}

	return nil
}

// broadcastMessageToOtherAvatars sends the avatar's message to other avatars' threads
func (w *AvatarWatcher) broadcastMessageToOtherAvatars(content string) error {
	if w.assistant == nil {
		log.Printf("[AvatarWatcher] Cannot broadcast: assistant is nil")
		return nil
	}

	// Get all avatars in the conversation with their thread IDs
	avatars, threadIDs, err := w.db.GetConversationAvatarsWithThreads(w.conversationID)
	if err != nil {
		return err
	}

	// Format the avatar's message for other avatars' threads
	formattedContent := logic.FormatAvatarMessage(w.avatar.Name, content)

	// Send to each other avatar's thread
	targetCount := 0
	for i, avatar := range avatars {
		// Skip self
		if avatar.ID == w.avatar.ID {
			continue
		}

		if i >= len(threadIDs) || threadIDs[i] == "" {
			log.Printf("[AvatarWatcher] Skipping avatar without thread_id conversation_id=%d avatar_id=%d avatar_name=%s",
				w.conversationID, avatar.ID, avatar.Name)
			continue
		}

		threadID := threadIDs[i]
		log.Printf("[AvatarWatcher] Broadcasting message to avatar thread conversation_id=%d from_avatar_id=%d from_avatar_name=%s to_avatar_id=%d to_avatar_name=%s thread_id=%s",
			w.conversationID, w.avatar.ID, w.avatar.Name, avatar.ID, avatar.Name, threadID)
		log.Printf("[AvatarWatcher] LLM Input thread_id=%s avatar_name=%s message_content=%q", threadID, avatar.Name, formattedContent)

		// Wait for any active runs to complete before adding message
		if err := w.assistant.WaitForActiveRunsToComplete(threadID, 30*time.Second); err != nil {
			log.Printf("[AvatarWatcher] Warning: timeout waiting for active runs thread_id=%s to_avatar_name=%s err=%v", threadID, avatar.Name, err)
		}

		_, err := w.assistant.CreateMessage(threadID, formattedContent)
		if err != nil {
			log.Printf("[AvatarWatcher] Warning: failed to send message to avatar thread thread_id=%s to_avatar_name=%s err=%v", threadID, avatar.Name, err)
			// Continue - try other avatars
		} else {
			log.Printf("[AvatarWatcher] Message sent to avatar thread successfully thread_id=%s to_avatar_name=%s", threadID, avatar.Name)
			targetCount++
		}
	}

	log.Printf("[AvatarWatcher] Broadcasting message to other avatars completed conversation_id=%d avatar_name=%s message_id=%d target_count=%d",
		w.conversationID, w.avatar.Name, 0, targetCount)

	return nil
}

// buildConversationContext builds context from recent messages for the run
func (w *AvatarWatcher) buildConversationContext() string {
	// Get recent messages from the conversation
	messages, err := w.db.GetMessages(w.conversationID)
	if err != nil {
		log.Printf("[AvatarWatcher] Failed to get messages for context conversation_id=%d err=%v",
			w.conversationID, err)
		return ""
	}

	if len(messages) == 0 {
		return ""
	}

	// Get avatar names for lookup
	avatars, err := w.db.GetConversationAvatars(w.conversationID)
	if err != nil {
		log.Printf("[AvatarWatcher] Failed to get avatars for context conversation_id=%d err=%v",
			w.conversationID, err)
		return ""
	}

	avatarNameMap := make(map[int64]string)
	for _, a := range avatars {
		avatarNameMap[a.ID] = a.Name
	}

	// Convert messages to format-ready structure
	var formatMessages []logic.MessageForFormat
	for _, msg := range messages {
		fm := logic.MessageForFormat{
			Content: msg.Content,
		}

		if msg.SenderType == models.SenderTypeUser {
			fm.SenderType = logic.SenderTypeUserFormat
			fm.SenderName = ""
		} else {
			fm.SenderType = logic.SenderTypeAvatarFormat
			if msg.SenderID != nil {
				if name, ok := avatarNameMap[*msg.SenderID]; ok {
					fm.SenderName = name
				}
			}
		}

		formatMessages = append(formatMessages, fm)
	}

	// Format message history excluding current avatar's messages
	formattedHistory := logic.FormatMessageHistory(formatMessages, w.avatar.Name)

	if formattedHistory == "" {
		return ""
	}

	// Build the additional context
	context := "【Conversation History】\n" +
		"The following are previous messages in this conversation.\n" +
		"Messages from you (assistant) are excluded. Respond based on this context.\n\n" +
		formattedHistory

	log.Printf("[AvatarWatcher] Built conversation context avatar=%s context_length=%d",
		w.avatar.Name, len(context))

	return context
}

// GetLastMessageID returns the last processed message ID (for testing)
func (w *AvatarWatcher) GetLastMessageID() int64 {
	return w.lastMessageID
}
