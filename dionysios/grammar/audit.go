package grammar

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/models"
)

type GrammarAuditEvent struct {
	At             time.Time `json:"at"`
	Step           string    `json:"step"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitempty"`
	Source         string    `json:"source,omitempty"`
	Rule           string    `json:"rule,omitempty"`
	SearchTerm     string    `json:"searchTerm,omitempty"`
	RootWord       string    `json:"rootWord,omitempty"`
	ResultCount    int       `json:"resultCount,omitempty"`
	CandidateCount int       `json:"candidateCount,omitempty"`
	Details        []string  `json:"details,omitempty"`
}

type GrammarAuditLog struct {
	RequestID      string              `json:"requestId,omitempty"`
	Word           string              `json:"word,omitempty"`
	StartedAt      time.Time           `json:"startedAt"`
	CompletedAt    time.Time           `json:"completedAt,omitempty"`
	DurationMs     int64               `json:"durationMs,omitempty"`
	Outcome        string              `json:"outcome,omitempty"`
	DecisionSource string              `json:"decisionSource,omitempty"`
	DecisionReason string              `json:"decisionReason,omitempty"`
	Events         []GrammarAuditEvent `json:"events"`
}

type GrammarAuditResponse struct {
	Results []models.Result `json:"results"`
	Audit   GrammarAuditLog `json:"audit"`
}

func newGrammarAuditLog(requestID, word string) *GrammarAuditLog {
	return &GrammarAuditLog{
		RequestID: requestID,
		Word:      word,
		StartedAt: time.Now(),
		Events:    []GrammarAuditEvent{},
	}
}

func (g *GrammarAuditLog) Add(event GrammarAuditEvent) {
	if g == nil {
		return
	}

	if event.At.IsZero() {
		event.At = time.Now()
	}

	g.Events = append(g.Events, event)
}

func (g *GrammarAuditLog) Complete(outcome, decisionSource, decisionReason string) {
	if g == nil {
		return
	}

	g.CompletedAt = time.Now()
	g.DurationMs = g.CompletedAt.Sub(g.StartedAt).Milliseconds()
	g.Outcome = outcome
	g.DecisionSource = decisionSource
	g.DecisionReason = decisionReason
}

func (g *GrammarAuditLog) Emit() {
	if g == nil {
		return
	}

	payload, err := json.Marshal(g)
	if err != nil {
		logging.Error(err.Error())
		return
	}

	logging.Info(string(payload))
}

func auditRequested(queryValue string) bool {
	audit, err := strconv.ParseBool(queryValue)
	if err == nil {
		return audit
	}

	return queryValue == "1"
}
