package scholar

import (
	"fmt"
	"github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/plato/logging"
)

func CreateIndexAtStartup(policyName, index string, elastic aristoteles.Client) error {
	indexMapping := createScholarIndexMapping(policyName)
	created, err := elastic.Index().Create(index, indexMapping)
	if err != nil {
		logging.Warn(fmt.Sprintf("index creation returned an error, this most likely means the index already exists: %s", err.Error()))
		return nil
	}

	logging.Info(fmt.Sprintf("created index: %s %v", index, created.Acknowledged))

	return nil
}

func createScholarIndexMapping(policyName string) map[string]interface{} {
	return map[string]interface{}{
		"settings": map[string]interface{}{
			"index": map[string]interface{}{
				"number_of_shards":   1,
				"number_of_replicas": 1,
				"lifecycle.name":     policyName, // Add this line to associate the policy
				"refresh_interval":   "30s",
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"rootWord": map[string]interface{}{
					"type": "text",
				},
				"unaccented": map[string]interface{}{
					"type": "text",
				},
				"variants": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"searchTerm": map[string]interface{}{
							"type": "text",
						},
						"score": map[string]interface{}{
							"type": "integer",
						},
					},
				},
				"partOfSpeech": map[string]interface{}{
					"type": "keyword",
				},
				"translations": map[string]interface{}{
					"type": "text",
				},
				"categories": map[string]interface{}{
					"type": "nested",
					"properties": map[string]interface{}{
						"forms": map[string]interface{}{
							"type": "nested",
							"properties": map[string]interface{}{
								"rule": map[string]interface{}{
									"type": "text",
								},
								"word": map[string]interface{}{
									"type": "text",
								},
							},
						},
					},
				},
			},
		},
	}
}
