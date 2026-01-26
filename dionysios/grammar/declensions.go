package grammar

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/models"
	erv1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	"github.com/odysseia-greek/alexandreia/eratosthenes/library"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var miscNames = []string{"adverb", "conjunction", "particle", "preposition"}
var exceptionList = map[string]bool{
	"λογάς": true,
}

// queryLibrary tries to find results for given words in the dictionary.
// It queries the Alexandros dictionary for the stripped word and returns the search results.
func (d *DionysosHandler) queryLibrary(ctx context.Context, term string) (*erv1.ResolveResponse, error) {
	outCtx, cancel := d.outgoingCtx(ctx)
	defer cancel()

	var grpcResponse *erv1.ResolveResponse

	request := &erv1.ResolveRequest{
		Term:  term,
		Limit: 5,
	}
	err := d.LibraryService.CallWithReconnect(func(client *library.LibraryClient) error {
		var innerErr error
		grpcResponse, innerErr = client.Resolve(outCtx, request)
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	return grpcResponse, nil
}

// removeAccents removes accents from a given string and returns the transformed string.
// It uses golang.org/x/text/transform package to normalize and remove combining diacritical marks.
func (d *DionysosHandler) removeAccents(s string) string {
	// Create a transformation chain to normalize and remove combining diacritical marks
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	// Apply the transformation to the input string
	output, _, e := transform.String(t, s)
	if e != nil {
		logging.Error(e.Error())
	}

	// Return the transformed string
	return output
}

// parseDictResults parses the dictionary hit and extracts the translation and article (if available).
// It splits the Greek term to extract the article and returns the translation and article strings.
func (d *DionysosHandler) parseDictResults(result *erv1.Candidate) (article string) {
	// Split the Greek term by comma
	greek := strings.Split(result.Lemma, ",")

	// Check if there is an article present in the Greek term
	if len(greek) > 1 {
		// Remove spaces from the article and assign it to the article variable
		article = strings.Replace(greek[1], " ", "", -1)
	}

	// Return the translation and article
	return
}

func (d *DionysosHandler) isAWordWithoutDeclensions(word string) (bool, *models.DeclensionElement) {
	for _, rules := range d.DeclensionConfig.Declensions {
		for _, m := range miscNames {
			if rules.Type == m {
				for _, declensionWord := range rules.Declensions {
					if word == d.removeAccents(declensionWord.Declension) {
						return true, &declensionWord
					}
				}
			}
		}
	}

	return false, nil
}

// StartFindingRules initiates the process of finding declension rules and translations for a given word.
// It returns the declension translation results.
func (d *DionysosHandler) StartFindingRules(ctx context.Context, word string) (*models.DeclensionTranslationResults, error) {
	var results models.DeclensionTranslationResults

	noDeclensionWord, form := d.isAWordWithoutDeclensions(d.removeAccents(word))

	if noDeclensionWord {
		// if there are searchTerms (for example articles and pronouns) use those else just use the word
		rootWord := word
		if len(form.SearchTerm) > 0 {
			rootWord = form.SearchTerm[0]
		}
		singleSearchResult, err := d.queryLibrary(ctx, rootWord)
		if err != nil {
			logging.Debug(fmt.Sprintf("single search result gave an error: %s", err.Error()))
		}

		if singleSearchResult != nil && len(singleSearchResult.Candidates) > 0 {
			// Parse the dictionary results and create result objects

			result := models.Result{
				Word:        word,
				Rule:        form.RuleName,
				RootWord:    rootWord,
				Translation: []string{},
			}
			for _, searchResult := range singleSearchResult.Candidates {
				for _, gloss := range searchResult.Glosses {
					result.Translation = append(result.Translation, gloss)
				}
			}
			results.Results = append(results.Results, result)
		}
	} else {
		// even if the word is found as being in the misc group it might still be both
		declensions, err := d.searchForDeclensions(word)
		if err != nil {
			return nil, err
		}

		// Separate first/second declensions from third declensions
		firstSecondDeclensions := []models.Rule{}
		thirdDeclensions := []models.Rule{}
		remainingDeclensions := []models.Rule{}

		for _, declension := range declensions.Rules {
			if declension.Type == "firstDeclension" || declension.Type == "secondDeclension" {
				firstSecondDeclensions = append(firstSecondDeclensions, declension)
			} else if declension.Type == "thirdDeclension" {
				thirdDeclensions = append(thirdDeclensions, declension)
			} else {
				remainingDeclensions = append(remainingDeclensions, declension)
			}
		}

		// Function to process declensions and query dictionary
		processDeclensions := func(declensions []models.Rule) []models.Result {
			var processedResults []models.Result

			for _, declension := range declensions {
				if len(declension.SearchTerms) > 0 {
					for _, term := range declension.SearchTerms {
						// Filter out potential words that have been found and are either α or ω which are exclamations
						if utf8.RuneCountInString(term) == 1 && term != "ὁ" {
							continue
						}

						//replace παρε with παρα for imperfect verbs

						dictionaryHits, err := d.queryLibrary(ctx, term)
						if err != nil {
							// Handle the error
							continue
						}

						if dictionaryHits == nil {
							continue
						}

						result := models.Result{
							Word:        word,
							Rule:        declension.Rule,
							RootWord:    term,
							Translation: []string{},
						}

						// Parse the dictionary results and create result objects
						for _, hit := range dictionaryHits.Candidates {

							if hit.Lemma != "" {
								result.RootWord = hit.Lemma
							}

							article := d.parseDictResults(hit)

							for _, gloss := range hit.Glosses {
								result.Translation = append(result.Translation, gloss)
							}

							translation := result.Translation[0]

							// Skip adding the result if it already exists with the same translation
							if len(processedResults) > 0 {
								if translation == "" {
									continue
								}
							}

							// Handle articles and filter the results accordingly
							if article != "" {
								switch article {
								case "ὁ":
									if !strings.Contains(declension.Rule, "masc") {
										continue
									}
								case "ἡ":
									if !strings.Contains(declension.Rule, "fem") {
										continue
									}
								case "τό":
									if !strings.Contains(declension.Rule, "neut") {
										continue
									}
								}
							}
						}

						// Append the result to the results slice
						if len(result.Translation) > 0 {
							processedResults = append(processedResults, result)
						}
					}
				}
			}

			return processedResults
		}

		// Function to check if a word is in the exception list
		isException := func(word string) bool {
			_, found := exceptionList[word]
			return found
		}

		// Process first/second declensions
		r := processDeclensions(firstSecondDeclensions)
		for _, res := range r {
			if !isException(res.RootWord) {
				results.Results = append(results.Results, res)
			}
		}

		// If no results found from first/second declensions, process third declensions
		if len(r) == 0 {
			r = processDeclensions(thirdDeclensions)
			for _, res := range r {
				results.Results = append(results.Results, res)
			}
		}

		// If no results found from third declensions, process remaining possibilities
		if len(remainingDeclensions) > 0 {
			others := processDeclensions(remainingDeclensions)
			for _, res := range others {
				results.Results = append(results.Results, res)
			}
		}
	}

	// Perform additional filtering and removal of redundant results
	if len(results.Results) > 1 {
		filteredResults := make([]models.Result, 0)
		seen := make(map[string]int) // Map to track unique combinations and their indices in filteredResults

		for _, result := range results.Results {
			// Create a key that represents the unique combination of Rule, Translation, and RootWord (with accents removed)
			key := result.Rule + "|" + result.Translation[0] + "|" + d.removeAccents(result.RootWord)

			if result.Translation[0] == "" ||
				(result.RootWord == "η" || result.RootWord == "ο" || result.RootWord == "το") && result.Word != result.RootWord {
				continue // Skip adding this result
			}

			if index, found := seen[key]; found {
				// Check if the current result has accents but the one in filteredResults does not
				if result.RootWord != d.removeAccents(result.RootWord) && filteredResults[index].RootWord == d.removeAccents(filteredResults[index].RootWord) {
					// Replace the no-accent version with the current result (which has accents)
					filteredResults[index] = result
				}
			} else {
				// Add the current result to filteredResults and record its index in seen
				filteredResults = append(filteredResults, result)
				seen[key] = len(filteredResults) - 1 // Store the index of this result in filteredResults
			}
		}

		// Replace the original results with the filtered results
		results.Results = filteredResults
	} else if len(results.Results) == 0 {
		dictionaryHits, err := d.queryLibrary(ctx, word)
		if err != nil {
			logging.Debug(fmt.Sprintf("single search result gave an error: %s", err.Error()))
		}

		if dictionaryHits != nil && len(dictionaryHits.Candidates) > 0 {
			result := models.Result{
				Word:        word,
				Rule:        "no rule found",
				RootWord:    word,
				Translation: []string{},
			}

			for _, hit := range dictionaryHits.Candidates {
				for _, gloss := range hit.Glosses {
					result.Translation = append(result.Translation, gloss)
				}

			}
			results.Results = append(results.Results, result)
		}
	}

	return &results, nil
}

// searchForDeclensions searches for declensions of a given word.
// It iterates over each declension and declension form, processes them, and returns the found declension rules.
func (d *DionysosHandler) searchForDeclensions(word string) (*models.FoundRules, error) {
	// Initialize the foundRules variable
	var foundRules models.FoundRules

	// Iterate over each declension
	for _, declension := range d.DeclensionConfig.Declensions {
		var contract bool

		// Determine if the declension type is "past" (where you might see contraction)
		switch declension.Type {
		case "past":
			contract = true
		case "article":
			var directRules models.FoundRules
			rules, directHit := d.loopOverIrregularVerbs(word, declension.Declensions)
			for _, rule := range rules.Rules {
				if directHit {
					directRules.Rules = append(directRules.Rules, rule)
				} else {
					foundRules.Rules = append(foundRules.Rules, rule)
				}
			}

			if directHit {
				return &directRules, nil
			}

			continue
		case "pronoun":
			var directRules models.FoundRules
			rules, directHit := d.loopOverIrregularVerbs(word, declension.Declensions)
			for _, rule := range rules.Rules {
				if directHit {
					directRules.Rules = append(directRules.Rules, rule)
				} else {
					foundRules.Rules = append(foundRules.Rules, rule)
				}
			}

			if directHit {
				return &directRules, nil
			}

			continue
		case "irregular":
			// Process irregular verbs separately and if the exact match is found we return here
			var directRules models.FoundRules
			rules, directHit := d.loopOverIrregularVerbs(word, declension.Declensions)
			for _, rule := range rules.Rules {
				if directHit {
					directRules.Rules = append(directRules.Rules, rule)
				} else {
					foundRules.Rules = append(foundRules.Rules, rule)
				}
			}

			if directHit {
				return &directRules, nil
			}

			continue
		default:
			contract = false
		}

		// Iterate over each declension form
		for _, declensionForm := range declension.Declensions {
			wordIsOfTypePare := false
			if len(word) >= 4 && strings.HasPrefix(d.removeAccents(word), "παρε") {
				contract = false
				wordIsOfTypePare = true
			}

			result := d.loopOverDeclensions(word, declensionForm, contract, declension.Name)
			// Check if any rules were found
			if len(result.Rules) >= 1 {
				for _, rule := range result.Rules {
					if wordIsOfTypePare {
						for index, searchTerm := range rule.SearchTerms {
							// Replace "παρε" with "παρα"
							parsedTerm := "παρα" + string([]rune(d.removeAccents(searchTerm))[4:])
							rule.SearchTerms[index] = parsedTerm
						}
					}
					d.addRuleIfDifferent(&foundRules, rule)
				}
			}
		}
	}

	// Return the found declension rules
	return &foundRules, nil
}

// loopOverDeclensions processes declensions for a given word and declension form.
// It checks if the declension form matches the word and returns the found declension rules.
func (d *DionysosHandler) loopOverDeclensions(word string, form models.DeclensionElement, prefix bool, declensionType string) models.FoundRules {
	// Initialize the declensions variable
	var declensions models.FoundRules

	// Determine the root cutoff based on the prefix flag
	rootCutOff := 0
	if prefix {
		rootCutOff = 1
	}

	// Remove accents from the declension and trim hyphens
	trimmedLetters := d.removeAccents(strings.Replace(form.Declension, "-", "", -1))

	// Determine the length of the declension
	lengthOfDeclension := utf8.RuneCountInString(trimmedLetters)

	// Remove accnets and convert the word to a rune slice
	// The accents can become a rune and so interfere with the rest of the function
	wordInRune := []rune(d.removeAccents(word))

	// If the length of the declension is greater than the word length, return empty declensions
	if lengthOfDeclension > len(wordInRune) || rootCutOff > len(wordInRune)-lengthOfDeclension {
		return declensions
	}

	// Extract the letters from the end of the word matching the length of the declension
	lettersOfWord := d.removeAccents(string(wordInRune[len(wordInRune)-lengthOfDeclension:]))

	// If the extracted letters match the trimmed declension, proceed with further checks
	if lettersOfWord == trimmedLetters {
		// Extract the root of the word based on the root cutoff
		rootOfWord := string(wordInRune[rootCutOff : len(wordInRune)-lengthOfDeclension])

		// Get the first letter of the word
		firstLetter := d.removeAccents(string(wordInRune[0]))

		// Initialize the words slice to store search terms
		var words []string

		// Iterate over each search term in the form's search terms
		for _, term := range form.SearchTerm {
			if prefix {
				// Handle prefix declensions
				legitimateStartLetters := []string{"η", "ε"}
				legitimate := false

				// Check if the first letter is one of the legitimate start letters
				for _, startLetter := range legitimateStartLetters {
					if startLetter == firstLetter {
						legitimate = true
					}
				}

				// Skip this search term if the first letter is not legitimate
				if !legitimate {
					continue
				}

				// Handle specific cases for "η" as the first letter
				if firstLetter == "η" {
					vowels := []string{"α", "ε"}

					// Append search terms for each vowel-root combination
					for _, vowel := range vowels {
						searchTerm := fmt.Sprintf("%s%s%s", vowel, rootOfWord, term)
						words = append(words, searchTerm)
					}

					continue
				}
			}

			// Append the search term with the root to the words slice
			searchTerm := fmt.Sprintf("%s%s", rootOfWord, term)
			words = append(words, searchTerm)
		}

		// Create a declension rule based on the form and append it to the declensions slice
		if len(words) >= 1 {
			declension := models.Rule{
				Rule:        form.RuleName,
				SearchTerms: words,
				Type:        declensionType,
			}
			declensions.Rules = append(declensions.Rules, declension)
		}
	}

	// Return the found declension rules
	return declensions
}

// loopOverIrregularVerbs processes irregular verbs for a given word.
// It checks if the stripped word matches the stripped outcome word and returns the found declension rules.
func (d *DionysosHandler) loopOverIrregularVerbs(word string, declensions []models.DeclensionElement) (models.FoundRules, bool) {
	// Initialize the rules variable
	var rules models.FoundRules

	// Remove accents from the word
	strippedWord := d.removeAccents(word)

	// Iterate over each declension outcome
	for _, outcome := range declensions {
		if word == outcome.Declension {
			rules.Rules = append(rules.Rules, models.Rule{
				Rule:        outcome.RuleName,
				SearchTerms: outcome.SearchTerm,
			})

			return rules, true
		}
		// Remove accents from the outcome word
		strippedOutcomeWord := d.removeAccents(outcome.Declension)

		// Check if the stripped word matches the stripped outcome word
		if strippedWord == strippedOutcomeWord {
			// Create a declension rule based on the outcome
			declension := models.Rule{
				Rule:        outcome.RuleName,
				SearchTerms: outcome.SearchTerm,
			}

			// Append the declension rule to the rules slice
			rules.Rules = append(rules.Rules, declension)
		}
	}

	// Return the found declension rules
	return rules, false
}

// containsRule checks if a given rule is already present in the rules slice.
// It compares both the rule string and the search terms to determine if they are the same.
func (d *DionysosHandler) containsRule(rules []models.Rule, rule models.Rule) bool {
	for _, r := range rules {
		if r.Rule == rule.Rule {
			if d.slicesEqual(r.SearchTerms, rule.SearchTerms) {
				return true
			}

			for _, term := range r.SearchTerms {
				for _, innerSearchTerm := range rule.SearchTerms {
					if term == innerSearchTerm {
						return true
					}
				}
			}

		}
	}
	return false
}

// slicesEqual compares two string slices and returns true if they are equal.
func (d *DionysosHandler) slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

// addRuleIfDifferent adds a rule to the foundRules.Rules slice only if it is not already present
// or if the search terms are different.
func (d *DionysosHandler) addRuleIfDifferent(foundRules *models.FoundRules, rule models.Rule) {
	if !d.containsRule(foundRules.Rules, rule) {
		foundRules.Rules = append(foundRules.Rules, rule)
	}
}
