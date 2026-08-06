package apeiron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	elastic "github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/delphi/aristides/diplomat"
)

type AnaximanderHandler struct {
	Index      string
	Created    int
	PolicyName string
	Elastic    elastic.Client
	Ambassador *diplomat.ClientAmbassador
}

func (a *AnaximanderHandler) DeleteIndexAtStartUp() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	deleted, err := a.Elastic.Index().DeleteWithContext(ctx, a.Index)
	logging.Info(fmt.Sprintf("deleted index: %s success: %v", a.Index, deleted))
	if err != nil {
		if deleted {
			return nil
		}
		if strings.Contains(err.Error(), "index_not_found_exception") {
			logging.Error(err.Error())
			return nil
		}

		return err
	}

	return nil
}

func (a *AnaximanderHandler) CreateIndexAtStartup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexMapping := a.Elastic.Builder().GrammarIndex(a.PolicyName)
	created, err := a.Elastic.Index().CreateWithContext(ctx, a.Index, indexMapping)
	if err != nil {
		return err
	}

	logging.Info(fmt.Sprintf("created index: %s %v", a.Index, created.Acknowledged))

	return nil
}

func (a *AnaximanderHandler) AddToElastic(declension models.Declension) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	upload, err := json.Marshal(declension)
	if err != nil {
		return err
	}

	doc, err := a.Elastic.Index().CreateDocumentWithContext(ctx, a.Index, upload)
	logging.Info(fmt.Sprintf("created document: %s %v", a.Index, doc))
	a.Created++
	if err != nil {
		return err
	}

	return nil
}

func (a *AnaximanderHandler) PrintProgress(total int) {
	for {
		percentage := float64(a.Created) / float64(total) * 100
		logging.Info(fmt.Sprintf("Progress: %d/%d documents created (%.2f%%)", a.Created, total, percentage))
		time.Sleep(1000 * time.Second)
	}
}
