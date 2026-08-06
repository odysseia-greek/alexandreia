package grammar

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/odysseia-greek/agora/plato/transform"
	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	sv1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const (
	envTextModeRateLimitWindow     = "DIONYSIOS_TEXT_MODE_RATE_LIMIT_WINDOW"
	defaultTextModeRateLimitWindow = 2 * time.Second
)

func (d *DionysosHandler) TextMode(ctx context.Context, request *v1.TextModeRequest) (*v1.TextModeResponse, error) {
	text := strings.TrimSpace(request.GetText())
	if text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	sessionID := strings.TrimSpace(request.GetSessionId())
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	words := tokenizeText(text)
	if len(words) == 0 {
		return nil, status.Error(codes.InvalidArgument, "text must contain at least one word")
	}

	upstreamIP := upstreamIPFromContext(ctx)
	window := textModeRateLimitWindow()
	nextAllowed, err := d.reserveTextModeRequest(sessionID, upstreamIP, window)
	if err != nil {
		return nil, err
	}

	requestID := requestIDFromContext(ctx)
	auditLog := newGrammarAuditLog(requestID, text)
	auditLog.Add(GrammarAuditEvent{
		Step:           "text.tokenize",
		Status:         "ok",
		Reason:         "text normalized into words",
		CandidateCount: len(words),
	})

	textSearch := d.findKnownText(ctx, text, len(words))
	auditLog.Add(GrammarAuditEvent{
		Step:        "text.search",
		Status:      textSearch.Status,
		Reason:      textSearch.Message,
		Source:      "kallimachos",
		ResultCount: int(textSearch.MatchCount),
	})

	tokens := make([]*v1.TextToken, 0, len(words))
	glosses := make([]string, 0, len(words))
	resolvedCount := 0
	for position, word := range words {
		token := &v1.TextToken{Token: word, Position: uint32(position)}
		wordAudit := newGrammarAuditLog(requestID, word)
		results, checkErr := d.checkGrammarResults(ctx, word, requestID, wordAudit)
		if checkErr != nil {
			if ctx.Err() != nil {
				return nil, status.FromContextError(ctx.Err()).Err()
			}
			token.Message = status.Convert(checkErr).Message()
			token.Gloss = word
			auditLog.Add(GrammarAuditEvent{
				Step:       "text.token.analyze",
				Status:     "unresolved",
				Reason:     token.Message,
				SearchTerm: word,
			})
		} else {
			token.Results = mapDeclensionResults(results.Results)
			token.Gloss = bestLiteralGloss(word, token.Results)
			if token.Gloss == "" {
				token.Gloss = word
			}
			token.Resolved = len(token.Results) > 0
			if token.Resolved {
				resolvedCount++
			}
			auditLog.Add(GrammarAuditEvent{
				Step:        "text.token.analyze",
				Status:      "ok",
				Reason:      "grammar candidates found",
				SearchTerm:  word,
				ResultCount: len(token.Results),
			})
		}
		tokens = append(tokens, token)
		glosses = append(glosses, token.Gloss)
	}

	auditLog.Complete("success", "grammar", fmt.Sprintf("resolved %d of %d tokens", resolvedCount, len(tokens)))
	auditLog.Emit()

	response := &v1.TextModeResponse{
		SessionId:          sessionID,
		OriginalText:       text,
		LiteralTranslation: strings.Join(glosses, " "),
		Tokens:             tokens,
		RateLimit: &v1.RateLimitInfo{
			UpstreamIp:      upstreamIP,
			WindowSeconds:   uint32(window.Seconds()),
			NextAllowedTime: nextAllowed.UTC().Format(time.RFC3339Nano),
		},
		TextSearch: textSearch,
	}
	if request.GetIncludeAudit() {
		response.Audit = mapGrammarAudit(auditLog)
	}
	return response, nil
}

func (d *DionysosHandler) findKnownText(ctx context.Context, text string, wordCount int) *v1.TextSearchStatus {
	search := &v1.TextSearchStatus{Query: text}
	if wordCount < 3 || wordCount > 50 {
		search.Status = "skipped"
		search.Message = "known-text search requires between 3 and 50 words"
		return search
	}
	if d.ScholarService == nil || d.ScholarService.Client == nil {
		search.Status = "unavailable"
		search.Message = "Kallimachos is not configured"
		return search
	}

	search.Searched = true
	outCtx, cancel := d.outgoingCtx(ctx)
	defer cancel()
	response, err := d.ScholarService.Client.FindText(outCtx, &sv1.FindTextRequest{Text: text, Limit: 5})
	if err != nil {
		search.Status = "unavailable"
		search.Message = fmt.Sprintf("Kallimachos text search failed: %s", status.Convert(err).Message())
		return search
	}

	return mapKnownTextResponse(text, response)
}

func mapKnownTextResponse(query string, response *sv1.FindTextResponse) *v1.TextSearchStatus {
	search := &v1.TextSearchStatus{
		Searched:   true,
		Query:      query,
		Found:      response.GetFound(),
		Message:    response.GetMessage(),
		MatchCount: response.GetMatchCount(),
		Matches:    mapResearchAnalyzeResults(response.GetMatches()),
		Status:     "not_found",
	}
	if search.Found {
		search.Status = "found"
	}
	return search
}

func tokenizeText(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsMark(r)
	})
}

func bestLiteralGloss(token string, results []*v1.DeclensionResult) string {
	bestScore := -1
	bestGloss := ""
	normalizedToken := normalizeGreekWord(token)
	for _, result := range results {
		if result == nil {
			continue
		}
		score := 0
		isArticle := strings.HasPrefix(strings.ToLower(strings.TrimSpace(result.GetRule())), "article")
		if normalizeGreekWord(result.GetRootWord()) == normalizedToken {
			score += 100
		}
		if isArticle {
			score += 80
		}
		for _, translation := range result.Translation {
			if translation = strings.TrimSpace(translation); translation != "" {
				gloss := compactDictionaryGloss(translation)
				if isArticle {
					gloss = "the"
				}
				if score > bestScore {
					bestScore = score
					bestGloss = gloss
				}
				break
			}
		}
	}
	return bestGloss
}

func normalizeGreekWord(word string) string {
	return strings.ToLower(transform.RemoveAccents(strings.TrimSpace(word)))
}

func compactDictionaryGloss(translation string) string {
	if index := strings.IndexAny(translation, ",;"); index >= 0 {
		translation = translation[:index]
	}
	translation = strings.TrimSpace(translation)
	for _, prefix := range []string{"a ", "an "} {
		if strings.HasPrefix(strings.ToLower(translation), prefix) {
			return strings.TrimSpace(translation[len(prefix):])
		}
	}
	return translation
}

func textModeRateLimitWindow() time.Duration {
	if value := strings.TrimSpace(os.Getenv(envTextModeRateLimitWindow)); value != "" {
		if window, err := time.ParseDuration(value); err == nil && window > 0 {
			return window
		}
	}
	return defaultTextModeRateLimitWindow
}

func (d *DionysosHandler) reserveTextModeRequest(sessionID, upstreamIP string, window time.Duration) (time.Time, error) {
	now := time.Now()
	d.TextModeMu.Lock()
	defer d.TextModeMu.Unlock()
	if d.TextModeSessions == nil {
		d.TextModeSessions = make(map[string]time.Time)
	}
	if d.TextModeIPs == nil {
		d.TextModeIPs = make(map[string]time.Time)
	}
	for identity, last := range d.TextModeSessions {
		if now.Sub(last) >= window {
			delete(d.TextModeSessions, identity)
		}
	}
	for identity, last := range d.TextModeIPs {
		if now.Sub(last) >= window {
			delete(d.TextModeIPs, identity)
		}
	}
	if last, exists := d.TextModeSessions[sessionID]; exists && now.Sub(last) < window {
		return time.Time{}, status.Errorf(codes.ResourceExhausted, "session_id is rate limited until %s", last.Add(window).UTC().Format(time.RFC3339Nano))
	}
	if last, exists := d.TextModeIPs[upstreamIP]; exists && now.Sub(last) < window {
		return time.Time{}, status.Errorf(codes.ResourceExhausted, "upstream IP is rate limited until %s", last.Add(window).UTC().Format(time.RFC3339Nano))
	}
	d.TextModeSessions[sessionID] = now
	d.TextModeIPs[upstreamIP] = now
	return now.Add(window), nil
}

func upstreamIPFromContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for _, key := range []string{"x-forwarded-for", "x-real-ip"} {
			if values := md.Get(key); len(values) > 0 {
				if value := strings.TrimSpace(strings.Split(values[0], ",")[0]); value != "" {
					return value
				}
			}
		}
	}
	if remote, ok := peer.FromContext(ctx); ok && remote.Addr != nil {
		if host, _, err := net.SplitHostPort(remote.Addr.String()); err == nil {
			return host
		}
		return remote.Addr.String()
	}
	return "unknown"
}
