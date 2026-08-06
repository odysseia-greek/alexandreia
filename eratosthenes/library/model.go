package library

type Lemma struct {
	ID                string              `json:"id,omitempty"`
	Headword          string              `json:"headword"`
	Original          string              `json:"original"`
	Greek             string              `json:"greek"`
	English           string              `json:"english"`
	Normalized        string              `json:"normalized,omitempty"`
	LinkedWord        string              `json:"linkedWord,omitempty"`
	PartOfSpeech      string              `json:"partOfSpeech,omitempty"`
	Article           string              `json:"article,omitempty"`
	Gender            string              `json:"gender,omitempty"`
	Noun              *NounInfo           `json:"noun,omitempty"`
	Verb              *VerbInfo           `json:"verb,omitempty"`
	QuickGlosses      []*LocalizedGloss   `json:"quickGlosses"`
	Definitions       []*Definition       `json:"definitions"`
	ModernConnections []*ModernConnection `json:"modernConnections"`
}

type LocalizedGloss struct {
	Language string `json:"language"`
	Gloss    string `json:"gloss"`
}

type Definition struct {
	Grade    int32      `json:"grade"`
	Meanings []*Meaning `json:"meanings"`
}

type Meaning struct {
	Language   string   `json:"language"`
	Definition string   `json:"definition"`
	Notes      []string `json:"notes"`
	Example    *string  `json:"example,omitempty"`
}

type ModernConnection struct {
	Term string `json:"term"`
	Note string `json:"note,omitempty"`
}

type NounInfo struct {
	Declension string `json:"declension,omitempty"`
	Genitive   string `json:"genitive,omitempty"`
}

type VerbInfo struct {
	PrincipalParts []string `json:"principalParts"`
}
