package scholar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/transform"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	v1ar "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"

	"io"
	"strings"
	"time"
)

const (
	ROOTWORD = "rootWord"
)

func (a *AggregatorServiceImpl) Health(context.Context, *v1.HealthRequest) (*v1.HealthResponse, error) {
	return &v1.HealthResponse{
		Health: true,
	}, nil
}

func (a *AggregatorServiceImpl) CreateNewEntry(stream v1.Aristarchos_CreateNewEntryServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&v1.AggregatorStreamResponse{
				Ack: "acknowledged",
			})
		}
		if err != nil {
			return err
		}

		go a.createOrUpdate(in)
	}
}

func (a *AggregatorServiceImpl) createOrUpdate(request *v1.AggregatorCreationRequest) {
	startTime := time.Now()
	splitID := strings.Split(request.TraceId, "+")

	traceCall := false
	var traceID, spanID string

	if len(splitID) >= 3 {
		traceCall = splitID[2] == "1"
	}

	if len(splitID) >= 1 {
		traceID = splitID[0]
	}
	if len(splitID) >= 2 {
		spanID = splitID[1]
	}

	parsedWord := transform.RemoveAccents(request.RootWord)

	createNewWord := false
	shouldQueries := []map[string]interface{}{
		{"match_phrase": map[string]string{"rootWordEntry": request.RootWord}}, // Match root word
		{"match_phrase": map[string]string{"unaccented": parsedWord}},          // Match unaccented word
	}

	// Add a match clause for each variant in the variants array
	shouldQueries = append(shouldQueries, map[string]interface{}{
		"nested": map[string]interface{}{
			"path": "variants",
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"should": []map[string]interface{}{
						{"match_phrase": map[string]string{"variants.searchTerm": request.RootWord}}},
				},
			},
		},
	})

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": shouldQueries,
			},
		},
	}
	response, err := a.Elastic.Query().Match(a.Index, query)

	if err != nil {
		if strings.Contains(err.Error(), "404") {
			createNewWord = true
		} else {
			logging.Error(err.Error())
			return
		}
	} else if len(response.Hits.Hits) == 0 {
		createNewWord = true
	}

	if traceCall {
		go func() {
			parsedQuery, _ := json.Marshal(query)
			hits := int64(0)
			took := int64(0)
			if response != nil {
				hits = response.Hits.Total.Value
				took = response.Took
			}
			dataBaseSpan := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       spanID,
				Kind: &v1ar.ObserveRequest_DbSpan{DbSpan: &v1ar.ObserveDbSpan{
					Action: "search",
					Query:  string(parsedQuery),
					Hits:   hits,
					TookMs: took,
				}},
			}

			err := streamer.Send(dataBaseSpan)
			if err != nil {
				logging.Error(fmt.Sprintf("error returned from tracer: %s", err.Error()))
			}
		}()
	}

	entry, err := a.mapAndHandleGrammaticalCategories(request)
	if err != nil {
		logging.Error(fmt.Sprintf("error returned from mapping: %s", err.Error()))
		return
	}

	if entry.Categories == nil {
		logging.Error(fmt.Sprintf("could not map the word %s to a workable form", parsedWord))
		return
	}

	if createNewWord {
		variant := Variant{
			SearchTerm: request.RootWord,
			Score:      1,
		}
		entry.Variants = append(entry.Variants, variant)
		entryAsJson, _ := json.Marshal(entry)
		createDocument, err := a.Elastic.Index().CreateDocument(a.Index, entryAsJson)
		if err != nil {
			logging.Error(err.Error())
			return
		}

		logging.Debug(fmt.Sprintf("created document with id: %s and rootWordEntry: %s", createDocument.ID, request.RootWord))
		return
	}

	jsonHit, _ := json.Marshal(response.Hits.Hits[0].Source)
	rootWordEntry, _ := UnmarshalRootWordEntry(jsonHit)

	for i, conjugation := range rootWordEntry.Categories {
		formFound := false
		for _, cform := range conjugation.Forms {
			if cform.Word == entry.Categories[0].Forms[0].Word {
				formFound = true
				break
			}
		}
		if !formFound {
			rootWordEntry.Categories[i].Forms = append(rootWordEntry.Categories[i].Forms, entry.Categories[0].Forms[0])
		}
		break
	}

	translationFound := false

	for _, trans := range rootWordEntry.Translations {
		if trans == request.Translation {
			translationFound = true
			break
		}
	}

	if !translationFound || rootWordEntry.Translations == nil {
		rootWordEntry.Translations = append(rootWordEntry.Translations, request.Translation)
	}

	if rootWordEntry.UnaccentedWord == "" {
		rootWordEntry.UnaccentedWord = transform.RemoveAccents(request.RootWord)
	}

	// see if the current variant exists
	thisVariantFound := false
	for i, variant := range rootWordEntry.Variants {
		if variant.SearchTerm == request.RootWord {
			rootWordEntry.Variants[i].Score = variant.Score + 1
			thisVariantFound = true
		}
	}

	// if not we add it the array
	if !thisVariantFound {
		variant := Variant{
			SearchTerm: request.RootWord,
			Score:      1,
		}

		rootWordEntry.Variants = append(rootWordEntry.Variants, variant)
	}

	// now a check is done which word should serve as the rootWord of this entry
	var highestScore int
	for _, variant := range rootWordEntry.Variants {
		if variant.Score > highestScore {
			highestScore = variant.Score
			rootWordEntry.RootWord = variant.SearchTerm
		}
	}

	entryAsJson, _ := json.Marshal(rootWordEntry)
	createDocument, err := a.Elastic.Document().Update(a.Index, response.Hits.Hits[0].ID, entryAsJson)
	if err != nil {
		logging.Error(err.Error())
		return
	}

	if traceCall {
		go func() {
			parabasis := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       comedy.GenerateSpanID(),
				Kind: &v1ar.ObserveRequest_Action{
					Action: &v1ar.ObserveAction{
						Action: "CloseSpan",
						TookMs: time.Since(startTime).Milliseconds(),
						Status: "updated document",
					},
				},
			}
			if err := streamer.Send(parabasis); err != nil {
				logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
			}
		}()
	}

	logging.Debug(fmt.Sprintf("updated document with id: %s and rootWordEntry: %s", createDocument.ID, request.RootWord))
	return
}

func (a *AggregatorServiceImpl) RetrieveEntry(ctx context.Context, request *v1.AggregatorRequest) (*v1.RootWordResponse, error) {
	startTime := time.Now()
	requestID, ok := ctx.Value(config.DefaultTracingName).(string)
	if !ok {
		requestID = "donot+trace+0"
	}

	splitID := strings.Split(requestID, "+")

	traceCall := false
	var traceID, spanID string

	if len(splitID) >= 3 {
		traceCall = splitID[2] == "1"
	}

	if len(splitID) >= 1 {
		traceID = splitID[0]
	}
	if len(splitID) >= 2 {
		spanID = splitID[1]
	}

	parsedWord := transform.RemoveAccents(request.RootWord)
	shouldQueries := []map[string]interface{}{
		{"match_phrase": map[string]string{"rootWordEntry": request.RootWord}}, // Match root word
		{"match_phrase": map[string]string{"unaccented": parsedWord}},          // Match unaccented word
	}

	// Add a match clause for each variant in the variants array
	shouldQueries = append(shouldQueries, map[string]interface{}{
		"nested": map[string]interface{}{
			"path": "variants",
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"should": []map[string]interface{}{
						{"match_phrase": map[string]string{"variants.searchTerm": request.RootWord}}},
				},
			},
		},
	})

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": shouldQueries,
			},
		},
	}

	response, err := a.Elastic.Query().Match(a.Index, query)

	if traceCall {
		go func() {
			hits := int64(0)
			took := int64(0)
			if response != nil {
				hits = response.Hits.Total.Value
				took = response.Took
			}
			parsedQuery, _ := json.Marshal(query)
			dataBaseSpan := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       spanID,
				Kind: &v1ar.ObserveRequest_DbSpan{DbSpan: &v1ar.ObserveDbSpan{
					Action: "search",
					Query:  string(parsedQuery),
					Hits:   hits,
					TookMs: took,
				}},
			}

			err := streamer.Send(dataBaseSpan)
			if err != nil {
				logging.Error(fmt.Sprintf("error returned from tracer: %s", err.Error()))
			}
		}()
	}

	if err != nil {
		return nil, err
	} else if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no entry can be found")
	}

	var responsev1 v1.RootWordResponse
	jsonHit, _ := json.Marshal(response.Hits.Hits[0].Source)
	rootWord, _ := UnmarshalRootWordEntry(jsonHit)

	responsev1.RootWord = rootWord.RootWord
	responsev1.Translations = rootWord.Translations
	responsev1.PartOfSpeech = mapCategoryToEnum(rootWord.PartOfSpeech)
	for _, conj := range rootWord.Categories {
		conjv1 := &v1.GrammaticalCategory{}

		for _, form := range conj.Forms {
			formv1 := &v1.GrammaticalForm{
				Word: form.Word,
				Rule: form.Rule,
			}
			conjv1.Forms = append(conjv1.Forms, formv1)
		}

		responsev1.Categories = append(responsev1.Categories, conjv1)
	}

	if traceCall {
		go func() {
			parabasis := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       comedy.GenerateSpanID(),
				Kind: &v1ar.ObserveRequest_Action{
					Action: &v1ar.ObserveAction{
						Action: "CloseSpan",
						TookMs: time.Since(startTime).Milliseconds(),
						Status: "updated document",
					},
				},
			}
			if err := streamer.Send(parabasis); err != nil {
				logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
			}
		}()
	}

	return &responsev1, nil
}

func (a *AggregatorServiceImpl) RetrieveSearchWords(ctx context.Context, request *v1.AggregatorRequest) (*v1.SearchWordResponse, error) {
	startTime := time.Now()
	requestID, ok := ctx.Value(config.DefaultTracingName).(string)
	if !ok {
		requestID = "donot+trace+0"
	}

	splitID := strings.Split(requestID, "+")

	traceCall := false
	var traceID, spanID string

	if len(splitID) >= 3 {
		traceCall = splitID[2] == "1"
	}

	if len(splitID) >= 1 {
		traceID = splitID[0]
	}
	if len(splitID) >= 2 {
		spanID = splitID[1]
	}

	parsedWord := transform.RemoveAccents(request.RootWord)
	request.RootWord = parsedWord

	query := a.Elastic.Builder().MatchQuery(ROOTWORD, request.RootWord)
	response, err := a.Elastic.Query().Match(a.Index, query)

	if err != nil {
		return nil, err
	} else if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no entry can be found")
	}

	if traceCall {
		go func() {
			parsedQuery, _ := json.Marshal(query)
			hits := int64(0)
			took := int64(0)
			if response != nil {
				hits = response.Hits.Total.Value
				took = response.Took
			}

			dataBaseSpan := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       spanID,
				Kind: &v1ar.ObserveRequest_DbSpan{DbSpan: &v1ar.ObserveDbSpan{
					Action: "search",
					Query:  string(parsedQuery),
					Hits:   hits,
					TookMs: took,
				}},
			}

			err := streamer.Send(dataBaseSpan)
			if err != nil {
				logging.Error(fmt.Sprintf("error returned from tracer: %s", err.Error()))
			}
		}()
	}

	var responsev1 v1.SearchWordResponse
	jsonHit, _ := json.Marshal(response.Hits.Hits[0].Source)
	rootWord, _ := UnmarshalRootWordEntry(jsonHit)

	for _, conj := range rootWord.Categories {
		for _, form := range conj.Forms {
			responsev1.Word = append(responsev1.Word, form.Word)
		}
	}

	if traceCall {
		go func() {
			parabasis := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       comedy.GenerateSpanID(),
				Kind: &v1ar.ObserveRequest_Action{
					Action: &v1ar.ObserveAction{
						Action: "CloseSpan",
						TookMs: time.Since(startTime).Milliseconds(),
						Status: "updated document",
					},
				},
			}
			if err := streamer.Send(parabasis); err != nil {
				logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
			}
		}()
	}

	return &responsev1, nil
}

func (a *AggregatorServiceImpl) RetrieveRootFromGrammarForm(ctx context.Context, request *v1.AggregatorRequest) (*v1.FormsResponse, error) {
	startTime := time.Now()
	requestID, ok := ctx.Value(config.DefaultTracingName).(string)
	if !ok {
		logging.Error("could not extract combinedId")
		requestID = "donot+trace+0"
	}

	splitID := strings.Split(requestID, "+")

	traceCall := false
	var traceID, spanID string

	if len(splitID) >= 3 {
		traceCall = splitID[2] == "1"
	}

	if len(splitID) >= 1 {
		traceID = splitID[0]
	}
	if len(splitID) >= 2 {
		spanID = splitID[1]
	}

	var responsev1 v1.FormsResponse
	responsev1.Word = request.RootWord
	parsedWord := transform.RemoveAccents(request.RootWord)
	responsev1.UnaccentedWord = parsedWord

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"nested": map[string]interface{}{
				"path": "categories.forms",
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []map[string]interface{}{
							{
								"match": map[string]interface{}{
									"categories.forms.word": fmt.Sprintf("%s", request.RootWord),
								},
							},
						},
					},
				},
			},
		},
	}
	response, err := a.Elastic.Query().Match(a.Index, query)

	if err != nil {
		return nil, err
	} else if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no entry can be found")
	}

	if traceCall {
		go func() {
			parsedQuery, _ := json.Marshal(query)
			hits := int64(0)
			took := int64(0)
			if response != nil {
				hits = response.Hits.Total.Value
				took = response.Took
			}

			dataBaseSpan := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       spanID,
				Kind: &v1ar.ObserveRequest_DbSpan{DbSpan: &v1ar.ObserveDbSpan{
					Action: "search",
					Query:  string(parsedQuery),
					Hits:   hits,
					TookMs: took,
				}},
			}

			err := streamer.Send(dataBaseSpan)
			if err != nil {
				logging.Error(fmt.Sprintf("error returned from tracer: %s", err.Error()))
			}
		}()
	}

	jsonHit, _ := json.Marshal(response.Hits.Hits[0].Source)

	logging.Debug(fmt.Sprintf("only taken the first hit: %s", string(jsonHit)))
	rootEntry, _ := UnmarshalRootWordEntry(jsonHit)

	responsev1.RootWord = rootEntry.RootWord
	responsev1.Translation = rootEntry.Translations
	responsev1.PartOfSpeech = rootEntry.PartOfSpeech
	for _, variant := range rootEntry.Variants {
		responsev1.Variants = append(responsev1.Variants, variant.SearchTerm)
	}

	for _, conj := range rootEntry.Categories {
		for _, form := range conj.Forms {
			wordFormForm := transform.RemoveAccents(form.Word)
			if wordFormForm == parsedWord {
				responsev1.Rule = form.Rule
				responsev1.Word = form.Word
			}
		}
	}

	if traceCall {
		go func() {
			parabasis := &v1ar.ObserveRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       comedy.GenerateSpanID(),
				Kind: &v1ar.ObserveRequest_Action{
					Action: &v1ar.ObserveAction{
						Action: "CloseSpan",
						TookMs: time.Since(startTime).Milliseconds(),
						Status: "updated document",
					},
				},
			}
			if err := streamer.Send(parabasis); err != nil {
				logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
			}
		}()
	}

	return &responsev1, nil
}

func (a *AggregatorServiceImpl) mapAndHandleGrammaticalCategories(request *v1.AggregatorCreationRequest) (*RootWordEntry, error) {
	// example: 3th sing - impf - ind - act
	// example: 1st plur - aor - ind - act
	// example: noun - plural - masc - nom
	// example: pres act part - sing - masc - nom
	// example: inf - pres - act
	var entry RootWordEntry

	entry.UnaccentedWord = transform.RemoveAccents(request.RootWord)
	entry.RootWord = request.RootWord
	entry.PartOfSpeech = mapEnumToCategory(request.PartOfSpeech)
	entry.Translations = []string{request.Translation}

	conjForm := GrammaticalForm{
		Word: request.Word,
		Rule: request.Rule,
	}
	conj := GrammaticalCategory{
		Forms: []GrammaticalForm{conjForm},
	}

	entry.Categories = append(entry.Categories, conj)

	return &entry, nil
}

func mapCategoryToEnum(category string) v1.PartOfSpeech {
	switch category {
	case "verb":
		return v1.PartOfSpeech_VERB
	case "noun":
		return v1.PartOfSpeech_NOUN
	case "participle":
		return v1.PartOfSpeech_PARTICIPLE
	case "preposition":
		return v1.PartOfSpeech_PREPOSITION
	case "adverb":
		return v1.PartOfSpeech_ADVERB
	case "article":
		return v1.PartOfSpeech_ARTICLE
	case "conjunction":
		return v1.PartOfSpeech_CONJUNCTION
	case "pronoun":
		return v1.PartOfSpeech_PRONOUN
	case "particle":
		return v1.PartOfSpeech_PARTICLE
	default:
		return v1.PartOfSpeech_UNKNOWN_CATEGORY
	}
}

func mapEnumToCategory(category v1.PartOfSpeech) string {
	switch category {
	case v1.PartOfSpeech_VERB:
		return "verb"
	case v1.PartOfSpeech_NOUN:
		return "noun"
	case v1.PartOfSpeech_PARTICIPLE:
		return "participle"
	case v1.PartOfSpeech_PREPOSITION:
		return "preposition"
	case v1.PartOfSpeech_ADVERB:
		return "adverb"
	case v1.PartOfSpeech_ARTICLE:
		return "article"
	case v1.PartOfSpeech_CONJUNCTION:
		return "conjunction"
	case v1.PartOfSpeech_PRONOUN:
		return "pronoun"
	case v1.PartOfSpeech_PARTICLE:
		return "particle"
	default:
		return "UNKNOWN"
	}
}
