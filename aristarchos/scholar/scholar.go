package scholar

import (
	"context"
	"encoding/json"
	"fmt"

	"io"
	"strings"

	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/transform"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
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
	ctx := stream.Context()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&v1.AggregatorStreamResponse{Ack: "acknowledged"})
		}
		if err != nil {
			return err
		}

		req := in

		go a.createOrUpdate(ctx, req)
	}
}

func (a *AggregatorServiceImpl) createOrUpdate(ctx context.Context, request *v1.AggregatorCreationRequest) error {
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
	response, err := a.Elastic.Query().MatchWithContext(ctx, a.Index, query)

	if err != nil {
		if strings.Contains(err.Error(), "404") {
			createNewWord = true
		} else {
			logging.Error(err.Error())
			return fmt.Errorf("query root word: %w", err)
		}
	} else if len(response.Hits.Hits) == 0 {
		createNewWord = true
	}

	if response != nil {
		go comedy.DatabaseSpan(query, response.Hits.Total.Value, response.Took, ctx, a.Streamer)
	}

	entry, err := a.mapAndHandleGrammaticalCategories(request)
	if err != nil {
		logging.Error(fmt.Sprintf("error returned from mapping: %s", err.Error()))
		return fmt.Errorf("map grammatical categories: %w", err)
	}

	if entry.Categories == nil {
		err := fmt.Errorf("could not map the word %s to a workable form", parsedWord)
		logging.Error(err.Error())
		return err
	}

	if createNewWord {
		variant := Variant{
			SearchTerm: request.RootWord,
			Score:      1,
		}
		entry.Variants = append(entry.Variants, variant)
		entryAsJson, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal new root word entry: %w", err)
		}
		createDocument, err := a.Elastic.Index().CreateDocumentWithContext(ctx, a.Index, entryAsJson)
		if err != nil {
			logging.Error(err.Error())
			return fmt.Errorf("create root word entry: %w", err)
		}

		logging.Debug(fmt.Sprintf("created document with id: %s and rootWordEntry: %s", createDocument.ID, request.RootWord))
		return nil
	}

	jsonHit, err := json.Marshal(response.Hits.Hits[0].Source)
	if err != nil {
		return fmt.Errorf("marshal existing root word entry: %w", err)
	}
	rootWordEntry, err := UnmarshalRootWordEntry(jsonHit)
	if err != nil {
		return fmt.Errorf("unmarshal existing root word entry: %w", err)
	}

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

	entryAsJson, err := json.Marshal(rootWordEntry)
	if err != nil {
		return fmt.Errorf("marshal updated root word entry: %w", err)
	}
	createDocument, err := a.Elastic.Document().UpdateWithContext(ctx, a.Index, response.Hits.Hits[0].ID, entryAsJson)
	if err != nil {
		logging.Error(err.Error())
		return fmt.Errorf("update root word entry: %w", err)
	}

	logging.Debug(fmt.Sprintf("updated document with id: %s and rootWordEntry: %s", createDocument.ID, request.RootWord))
	return nil
}

func (a *AggregatorServiceImpl) RetrieveEntry(ctx context.Context, request *v1.AggregatorRequest) (*v1.RootWordResponse, error) {
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

	response, err := a.Elastic.Query().MatchWithContext(ctx, a.Index, query)

	if err != nil {
		return nil, err
	} else if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no entry can be found")
	}

	go comedy.DatabaseSpan(query, response.Hits.Total.Value, response.Took, ctx, a.Streamer)

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

	return &responsev1, nil
}

func (a *AggregatorServiceImpl) RetrieveSearchWords(ctx context.Context, request *v1.AggregatorRequest) (*v1.SearchWordResponse, error) {

	parsedWord := transform.RemoveAccents(request.RootWord)
	request.RootWord = parsedWord

	query := a.Elastic.Builder().MatchQuery(ROOTWORD, request.RootWord)
	response, err := a.Elastic.Query().MatchWithContext(ctx, a.Index, query)

	if err != nil {
		return nil, err
	} else if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no entry can be found")
	}

	go comedy.DatabaseSpan(query, response.Hits.Total.Value, response.Took, ctx, a.Streamer)

	var responsev1 v1.SearchWordResponse
	jsonHit, _ := json.Marshal(response.Hits.Hits[0].Source)
	rootWord, _ := UnmarshalRootWordEntry(jsonHit)

	for _, conj := range rootWord.Categories {
		for _, form := range conj.Forms {
			responsev1.Word = append(responsev1.Word, form.Word)
		}
	}

	return &responsev1, nil
}

func (a *AggregatorServiceImpl) RetrieveRootFromGrammarForm(ctx context.Context, request *v1.AggregatorRequest) (*v1.FormsResponse, error) {
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
	response, err := a.Elastic.Query().MatchWithContext(ctx, a.Index, query)

	if err != nil {
		return nil, err
	} else if len(response.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no entry can be found")
	}

	go comedy.DatabaseSpan(query, response.Hits.Total.Value, response.Took, ctx, a.Streamer)

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
