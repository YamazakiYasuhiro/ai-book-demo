package logic

import (
	"sync"

	"multi-avatar-chat/internal/models"
)

// DiscussionConfig configures the discussion mode behavior
type DiscussionConfig struct {
	// MaxResponses is the maximum number of avatar responses before stopping
	MaxResponses int

	// EnableChaining allows avatars to respond to each other
	EnableChaining bool

	// ExcludeLastSender prevents the same avatar from responding twice in a row
	ExcludeLastSender bool
}

// DefaultDiscussionConfig returns the default configuration
func DefaultDiscussionConfig() DiscussionConfig {
	return DiscussionConfig{
		MaxResponses:      5,
		EnableChaining:    true,
		ExcludeLastSender: true,
	}
}

// DiscussionMode manages the state of avatar discussions
type DiscussionMode struct {
	config          DiscussionConfig
	running         bool
	responseCount   int
	lastResponder   *models.Avatar
	responseHistory []models.Avatar
	mu              sync.RWMutex
}

// NewDiscussionMode creates a new discussion mode manager
func NewDiscussionMode(config DiscussionConfig) *DiscussionMode {
	return &DiscussionMode{
		config:          config,
		responseHistory: make([]models.Avatar, 0),
	}
}

// Start begins the discussion mode
func (dm *DiscussionMode) Start() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.running = true
}

// Stop ends the discussion mode
func (dm *DiscussionMode) Stop() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.running = false
}

// IsRunning returns whether discussion mode is active
func (dm *DiscussionMode) IsRunning() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.running
}

// CanContinue returns whether the discussion can continue
func (dm *DiscussionMode) CanContinue() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if !dm.running {
		return false
	}

	if !dm.config.EnableChaining {
		return dm.responseCount == 0
	}

	return dm.responseCount < dm.config.MaxResponses
}

// RecordResponse records an avatar's response
func (dm *DiscussionMode) RecordResponse(avatar models.Avatar) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.responseCount++
	avatarCopy := avatar
	dm.lastResponder = &avatarCopy
	dm.responseHistory = append(dm.responseHistory, avatar)
}

// GetResponseCount returns the number of responses so far
func (dm *DiscussionMode) GetResponseCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.responseCount
}

// GetLastResponder returns the last avatar that responded
func (dm *DiscussionMode) GetLastResponder() *models.Avatar {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.lastResponder
}

// GetResponseHistory returns the list of avatars that have responded
func (dm *DiscussionMode) GetResponseHistory() []models.Avatar {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	history := make([]models.Avatar, len(dm.responseHistory))
	copy(history, dm.responseHistory)
	return history
}

// GetNextResponder selects the next avatar to respond
// Returns nil if no suitable avatar is available
func (dm *DiscussionMode) GetNextResponder(avatars []models.Avatar) *models.Avatar {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if len(avatars) == 0 {
		return nil
	}

	// Filter out last responder if configured
	var candidates []models.Avatar
	for _, avatar := range avatars {
		if dm.config.ExcludeLastSender && dm.lastResponder != nil && avatar.ID == dm.lastResponder.ID {
			continue
		}
		candidates = append(candidates, avatar)
	}

	if len(candidates) == 0 {
		return nil
	}

	// Return the first candidate (could be randomized or based on other criteria)
	return &candidates[0]
}

// Reset resets the discussion state
func (dm *DiscussionMode) Reset() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.responseCount = 0
	dm.lastResponder = nil
	dm.responseHistory = make([]models.Avatar, 0)
}

// ShouldResponderContinue determines if a specific avatar should continue responding
// based on the discussion context
func (dm *DiscussionMode) ShouldResponderContinue(avatar models.Avatar, content string, allAvatars []models.Avatar) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if !dm.running || !dm.config.EnableChaining {
		return false
	}

	if dm.responseCount >= dm.config.MaxResponses {
		return false
	}

	// Check if the avatar was mentioned in the last response
	avatarNames := make([]string, len(allAvatars))
	for i, a := range allAvatars {
		avatarNames[i] = a.Name
	}

	mentions := ParseMentions(content)
	matched := MatchAvatarNames(mentions, avatarNames)

	for _, name := range matched {
		if name == avatar.Name {
			return true
		}
	}

	return false
}

