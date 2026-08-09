package agents

import (
	"strings"
)

// EscalationAgent detects frustrated or complex queries that need human support
type EscalationAgent struct{}

// NewEscalationAgent creates an escalation detector
func NewEscalationAgent() *EscalationAgent {
	return &EscalationAgent{}
}

// ShouldEscalate returns true if the query signals frustration or a need for human help
func (ea *EscalationAgent) ShouldEscalate(query string) bool {
	lower := strings.ToLower(query)
	frustrationSignals := []string{
		"angry", "frustrated", "annoyed", "terrible", "worst",
		"human", "representative", "agent", "speak to someone",
		"refund", "complaint", "broken", "wrong order",
	}
	for _, signal := range frustrationSignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

// EscalationMessage returns a warm handoff message
func (ea *EscalationAgent) EscalationMessage() string {
	return "I'm connecting you with a stylist who can help right away. Please hold for just a moment 💕"
}
