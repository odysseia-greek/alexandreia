package grammar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseia-greek/agora/plato/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func declensionConfigFromFixture(t *testing.T, names ...string) *models.DeclensionConfig {
	t.Helper()

	config := &models.DeclensionConfig{}
	for _, name := range names {
		payload, err := os.ReadFile(filepath.Join("testdata", name+".json"))
		require.NoError(t, err)

		var declension models.Declension
		require.NoError(t, json.Unmarshal(payload, &declension))
		config.Declensions = append(config.Declensions, declension)
	}
	return config
}

func TestGrammarInputLowercasesCapitalizedGreekWithoutMovingAccent(t *testing.T) {
	assert.Equal(t, "ἀρχὴ", normalizeGrammarInput("Ἀρχὴ"))
}

func TestExactArticleWinsOverAccentInsensitiveConjunction(t *testing.T) {
	handler := DionysosHandler{DeclensionConfig: models.DeclensionConfig{Declensions: []models.Declension{
		{
			Type: "conjunction",
			Declensions: []models.DeclensionElement{
				{Declension: "ἤ", RuleName: "conjunction", SearchTerm: []string{"ἤ"}},
			},
		},
		{
			Type: "article",
			Declensions: []models.DeclensionElement{
				{Declension: "ἡ", RuleName: "article - sing - fem - nom", SearchTerm: []string{"ὁ"}},
			},
		},
	}}}

	isMisc, form := handler.isAWordWithoutDeclensions("ἡ")
	assert.False(t, isMisc)
	assert.Nil(t, form)
}

func TestExactAccentedEndingDistinguishesNominativeFromDative(t *testing.T) {
	assert.True(t, matchesExactEnding("ἀρχή", "-ή"))
	assert.False(t, matchesExactEnding("ἀρχή", "-ῃ"))
	assert.True(t, matchesExactEnding("ἀρχῇ", "-ῇ"))
}

func TestFeminineGenitiveCanReconstructFinalSigmaLemma(t *testing.T) {
	handler := DionysosHandler{}
	form := models.DeclensionElement{
		Declension: "-ης",
		RuleName:   "noun - sing - fem - gen",
		SearchTerm: []string{""},
	}

	rules := handler.loopOverDeclensions("πάσης", form, false, "firstDeclension")
	require.Len(t, rules.Rules, 1)
	require.Len(t, rules.Rules[0].SearchTerms, 1)
	assert.Equal(t, "πας", normalizeDictionaryTerm(rules.Rules[0].SearchTerms[0]))
	assert.Equal(t, "πᾶς", canonicalizeFinalSigma("πᾶσ"))
}

func TestContractedPresentMiddleInfinitiveReconstructsEpsilonContractLemma(t *testing.T) {
	handler := DionysosHandler{}
	form := models.DeclensionElement{
		Declension: "-εῖσθαι",
		RuleName:   "inf - pres - mid (contracted -έω)",
		SearchTerm: []string{"έω"},
	}

	rules := handler.loopOverDeclensions("αἱρεῖσθαι", form, false, "infinitive")
	require.Len(t, rules.Rules, 1)
	require.Len(t, rules.Rules[0].SearchTerms, 1)
	assert.Equal(t, "αιρέω", rules.Rules[0].SearchTerms[0])
}

func TestCheckGrammarEndPointIrregularVerb(t *testing.T) {
	numberOfRules := 1

	t.Run("HappyPathIrregularVerb", func(t *testing.T) {
		searchWord := "ἦσαν"
		expected := "3rd plural - impf - ind - act"
		expectedSearchResult := "εἰμί"

		declensionConfig := declensionConfigFromFixture(t, "irregular")

		handler := DionysosHandler{}

		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case "irregular":
				rules, _ := handler.loopOverIrregularVerbs(searchWord, declension.Declensions)
				for _, rule := range rules.Rules {
					foundRules.Rules = append(foundRules.Rules, rule)
				}

			default:
				continue
			}
		}

		assert.True(t, len(foundRules.Rules) == numberOfRules)
		expectedRuleFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
			}
			assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
		}
		assert.True(t, expectedRuleFound)
	})

	t.Run("FullPathIrregularVerb", func(t *testing.T) {
		searchWord := "ἦσαν"
		expected := "3rd plural - impf - ind - act"
		expectedSearchResult := "εἰμί"

		declensionConfig := declensionConfigFromFixture(t, "irregular")

		handler := DionysosHandler{
			DeclensionConfig: *declensionConfig,
		}

		foundRules, err := handler.searchForDeclensions(searchWord)
		assert.Nil(t, err)

		assert.Nil(t, err)
		assert.Len(t, foundRules.Rules, numberOfRules)
		expectedRuleFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
				assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
			}
		}
		assert.True(t, expectedRuleFound)
	})
}

func TestDeclensionImperfectumResult(t *testing.T) {
	numberOfRules := 2
	contraction := true

	t.Run("HappyPathImperfectum", func(t *testing.T) {
		searchWord := "ἔφερον"
		expected := "1st sing - impf - ind - act"
		expectedSearchResult := "φερω"

		declensionConfig := declensionConfigFromFixture(t, "imperfect")

		handler := DionysosHandler{}

		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case "imperfect":
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		assert.Equal(t, numberOfRules, len(foundRules.Rules))
		expectedRuleFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
			}
			assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
		}
		assert.True(t, expectedRuleFound)

	})
}

func TestDeclensionAoristResult(t *testing.T) {
	numberOfRules := 1
	multipleSearchResults := 4
	contraction := true
	name := "aorist"

	t.Run("HappyPathFirstAoristPsi", func(t *testing.T) {
		searchWord := "ἔγρᾰψᾰ"
		expected := "1st sing - aorist - ind - act"
		expectedSearchResult := "γραφω"

		declensionConfig := declensionConfigFromFixture(t, "aorist")

		handler := DionysosHandler{}
		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case name:
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		require.Len(t, foundRules.Rules, numberOfRules)
		assert.Equal(t, expected, foundRules.Rules[0].Rule)
		assert.Equal(t, expectedSearchResult, foundRules.Rules[0].SearchTerms[0])
	})

	t.Run("HappyPathFirstAoristSigma", func(t *testing.T) {
		searchWord := "ἐλύσαμεν"
		expected := "1st plural - aorist - ind - act"
		expectedSearchResult := "λυω"

		declensionConfig := declensionConfigFromFixture(t, "aorist")

		handler := DionysosHandler{}
		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case name:
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		require.Len(t, foundRules.Rules, numberOfRules)
		assert.Equal(t, expected, foundRules.Rules[0].Rule)
		assert.Equal(t, expectedSearchResult, foundRules.Rules[0].SearchTerms[0])
	})

	t.Run("HappyPathFirstAoristKappa", func(t *testing.T) {
		searchWord := "ἔπλέξεν"
		expected := "3th sing - aorist - ind - act"
		expectedSearchResult := "πλεκω"

		declensionConfig := declensionConfigFromFixture(t, "aorist")

		handler := DionysosHandler{}
		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case name:
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		require.Len(t, foundRules.Rules, numberOfRules)
		assert.Equal(t, expected, foundRules.Rules[0].Rule)
		assert.Equal(t, multipleSearchResults, len(foundRules.Rules[0].SearchTerms))
		found := false
		for _, searchTerm := range foundRules.Rules[0].SearchTerms {
			if searchTerm == expectedSearchResult {
				found = true
			}
		}

		assert.True(t, found)
	})

	t.Run("HappyPathFirstAoristSigmaKappa", func(t *testing.T) {
		searchWord := "ἐδῐδᾰ́ξᾰτε"
		expected := "2nd plural - aorist - ind - act"
		expectedSearchResult := "διδασκω"

		declensionConfig := declensionConfigFromFixture(t, "aorist")

		handler := DionysosHandler{}
		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case name:
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}
		require.Len(t, foundRules.Rules, numberOfRules)
		assert.Equal(t, expected, foundRules.Rules[0].Rule)
		assert.Equal(t, multipleSearchResults, len(foundRules.Rules[0].SearchTerms))
		found := false
		for _, searchTerm := range foundRules.Rules[0].SearchTerms {
			if searchTerm == expectedSearchResult {
				found = true
			}
		}

		assert.True(t, found)
	})

	t.Run("HappyPathFirstAoristGamma", func(t *testing.T) {
		searchWord := "ἔλεξᾰν"
		expected := "3th plural - aorist - ind - act"
		expectedSearchResult := "λεγω"

		declensionConfig := declensionConfigFromFixture(t, "aorist")

		handler := DionysosHandler{}
		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case name:
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		require.Len(t, foundRules.Rules, numberOfRules)
		assert.Equal(t, expected, foundRules.Rules[0].Rule)
		assert.Equal(t, multipleSearchResults, len(foundRules.Rules[0].SearchTerms))
		found := false
		for _, searchTerm := range foundRules.Rules[0].SearchTerms {
			if searchTerm == expectedSearchResult {
				found = true
			}
		}

		assert.True(t, found)
	})

	t.Run("HappyPathFirstAoristChiWithEta", func(t *testing.T) {
		searchWord := "ἦρξᾰς"
		expected := "2nd sing - aorist - ind - act"
		expectedSearchResult := "αρχω"

		declensionConfig := declensionConfigFromFixture(t, "aorist")

		handler := DionysosHandler{}
		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Name {
			case name:
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		require.Len(t, foundRules.Rules, numberOfRules)
		assert.Equal(t, expected, foundRules.Rules[0].Rule)
		assert.Equal(t, 2*multipleSearchResults, len(foundRules.Rules[0].SearchTerms))
		found := false
		for _, searchTerm := range foundRules.Rules[0].SearchTerms {
			if searchTerm == expectedSearchResult {
				found = true
			}
		}

		assert.True(t, found)
	})
}

func TestDeclensionParticiplesResult(t *testing.T) {
	numberOfRules := 1
	contraction := false

	t.Run("HappyPathParticpleMascSingNom", func(t *testing.T) {
		searchWord := "λυων"
		expected := "pres act part - sing - masc - nom"
		expectedSearchResult := "λυω"

		declensionConfig := declensionConfigFromFixture(t, "participle")

		handler := DionysosHandler{}

		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Type {
			case "participle":
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		assert.Equal(t, numberOfRules, len(foundRules.Rules))
		expectedRuleFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
			}
			assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
		}
		assert.True(t, expectedRuleFound)
	})

	t.Run("HappyPathParticpleFemDatPlural", func(t *testing.T) {
		searchWord := "λυοὐσαις"
		expected := "pres act part - plural - fem - dat"
		expectedSearchResult := "λυω"

		declensionConfig := declensionConfigFromFixture(t, "participle")

		handler := DionysosHandler{}

		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Type {
			case "participle":
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		assert.Equal(t, numberOfRules, len(foundRules.Rules))
		expectedRuleFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
			}
			assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
		}
		assert.True(t, expectedRuleFound)
	})

	t.Run("HappyPathParticpleNeutGenSing", func(t *testing.T) {
		searchWord := "λυὀντος"
		expected := "pres act part - sing - neut - gen"
		expectedSearchResult := "λυω"

		declensionConfig := declensionConfigFromFixture(t, "participle")

		handler := DionysosHandler{}

		var foundRules models.FoundRules

		for _, declension := range declensionConfig.Declensions {
			switch declension.Type {
			case "participle":
				for _, element := range declension.Declensions {
					rules := handler.loopOverDeclensions(searchWord, element, contraction, "")
					for _, rule := range rules.Rules {
						foundRules.Rules = append(foundRules.Rules, rule)
					}
				}

			default:
				continue
			}
		}

		assert.Equal(t, 2, len(foundRules.Rules))
		expectedRuleFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
			}
			assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
		}
		assert.True(t, expectedRuleFound)
	})

	t.Run("HappyPathParticpleAndVerbaResult", func(t *testing.T) {
		searchWord := "λυουσι"
		expected := "pres act part - plural - masc - dat"
		expectedVerba := "3rd plural - pres - ind - act"
		expectedSearchResult := "λυω"

		declensionConfig := declensionConfigFromFixture(t, "participle", "present")

		handler := DionysosHandler{
			DeclensionConfig: *declensionConfig,
		}

		foundRules, err := handler.searchForDeclensions(searchWord)
		assert.Nil(t, err)

		assert.Len(t, foundRules.Rules, 2)
		expectedRuleFound := false
		expectedVerbaFound := false
		for _, rule := range foundRules.Rules {
			if rule.Rule == expected {
				expectedRuleFound = true
			}

			if rule.Rule == expectedVerba {
				expectedVerbaFound = true
			}
			assert.Equal(t, expectedSearchResult, rule.SearchTerms[0])
		}
		assert.True(t, expectedRuleFound)
		assert.True(t, expectedVerbaFound)
	})
}
