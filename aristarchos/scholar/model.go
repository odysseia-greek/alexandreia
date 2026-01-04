package scholar

import "encoding/json"

type GrammaticalForm struct {
	Word string `json:"word"`
	Rule string `json:"rule"`
}

type GrammaticalCategory struct {
	Forms []GrammaticalForm `json:"forms"`
}

type RootWordEntry struct {
	RootWord       string                `json:"rootWord"`
	PartOfSpeech   string                `json:"partOfSpeech"` //  E.g., "verb", "noun", "participle"
	Translations   []string              `json:"translations"`
	UnaccentedWord string                `json:"unaccented"`
	Variants       []Variant             `json:"variants"`
	Categories     []GrammaticalCategory `json:"categories"`
}

type Variant struct {
	SearchTerm string `json:"searchTerm"`
	Score      int    `json:"score"`
}

func UnmarshalRootWordEntry(data []byte) (RootWordEntry, error) {
	var r RootWordEntry
	err := json.Unmarshal(data, &r)
	return r, err
}
