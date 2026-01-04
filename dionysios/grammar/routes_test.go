package grammar

import (
	"encoding/json"
	"fmt"
	"github.com/odysseia-greek/agora/archytas"
	elastic "github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/agora/plato/service"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	elasticIndexDefault = "grammar"
)

func TestPingPong(t *testing.T) {
	t.Run("Pass", func(t *testing.T) {
		testConfig := DionysosHandler{}
		router := InitRoutes(&testConfig)
		expected := "{\"result\":\"pong\"}"

		w := performGetRequest(router, "/dionysios/v1/ping")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, expected, w.Body.String())
	})
}

func TestHealthEndPoint(t *testing.T) {
	t.Run("Pass", func(t *testing.T) {
		fixtureFile := "info"
		mockCode := 200
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic: mockElasticClient,
		}

		router := InitRoutes(&testConfig)
		response := performGetRequest(router, "/dionysios/v1/health")

		var healthModel models.Health
		err = json.NewDecoder(response.Body).Decode(&healthModel)
		assert.Nil(t, err)
		//models.Health
		assert.Equal(t, http.StatusOK, response.Code)
		assert.True(t, healthModel.Healthy)
	})

	t.Run("Fail", func(t *testing.T) {
		fixtureFile := "infoServiceDown"
		mockCode := 502
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic: mockElasticClient,
		}

		router := InitRoutes(&testConfig)
		response := performGetRequest(router, "/dionysios/v1/health")

		var healthModel models.Health
		err = json.NewDecoder(response.Body).Decode(&healthModel)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusBadGateway, response.Code)
		assert.False(t, healthModel.Healthy)
	})
}

func TestCheckGrammarEndPointNouns(t *testing.T) {
	scheme := "http"
	baseUrl := "somelocalhost.com"

	config := service.ClientConfig{
		Ca: nil,
		Alexandros: service.OdysseiaApi{
			Url:    baseUrl,
			Scheme: scheme,
			Cert:   nil,
		},
	}

	t.Run("HappyPathMascSecond", func(t *testing.T) {
		fixtureFile := "dionysosMascNoun"
		mockCode := 200
		expected := "noun - plural - masc - nom"
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		codes := []int{
			200,
		}

		meroi := []models.Meros{
			{
				Greek:   "πόλεμος –ου, ὁ",
				English: "war",
			},
		}

		jsonString, err := json.Marshal(meroi)
		assert.Nil(t, err)

		responses := []string{string(jsonString)}

		testClient, err := service.NewFakeClient(config, codes, responses)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
			Client:           testClient,
		}
		router := InitRoutes(&testConfig)
		response := performGetRequest(router, "/dionysios/v1/checkGrammar?word=πόλεμοι")

		var declensions models.DeclensionTranslationResults
		err = json.NewDecoder(response.Body).Decode(&declensions)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.True(t, len(declensions.Results) == 1)
		assert.Equal(t, expected, declensions.Results[0].Rule)
	})

	t.Run("HappyPathPreposition", func(t *testing.T) {
		fixtureFile := "dionysosPreposition"
		mockCode := 200
		expected := "particle"
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		codes := []int{
			200,
		}

		meroi := []models.Meros{
			{
				Greek:   "ιςθεηφςσεφξκ",
				English: "something silly",
			},
		}

		jsonString, err := json.Marshal(meroi)
		assert.Nil(t, err)

		responses := []string{string(jsonString)}

		testClient, err := service.NewFakeClient(config, codes, responses)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
			Client:           testClient,
		}
		router := InitRoutes(&testConfig)
		response := performGetRequest(router, "/dionysios/v1/checkGrammar?word=ιςθεηφςσεφξκ")

		var declensions models.DeclensionTranslationResults
		err = json.NewDecoder(response.Body).Decode(&declensions)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		for _, decl := range declensions.Results {
			assert.Equal(t, expected, decl.Rule)
		}
	})

	t.Run("NoQueryParam", func(t *testing.T) {
		expected := "cannot be empty"

		fixtureFile := "dionysosPreposition"
		mockCode := 200
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
		}

		router := InitRoutes(&testConfig)
		response := performGetRequest(router, "/dionysios/v1/checkGrammar?word=")

		var validation models.ValidationError
		err = json.NewDecoder(response.Body).Decode(&validation)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Equal(t, expected, validation.Messages[0].Message)
	})
}

func TestCheckGrammarEndPointVerbaPresent(t *testing.T) {
	scheme := "http"
	baseUrl := "somelocalhost.com"

	config := service.ClientConfig{
		Ca: nil,
		Alexandros: service.OdysseiaApi{
			Url:    baseUrl,
			Scheme: scheme,
			Cert:   nil,
		},
	}

	t.Run("HappyPathPresentVerbaThirdPlurMi", func(t *testing.T) {
		searchWord := "ἀγαπᾰ́ουσῐ"
		fixtureFile := "dionysosVerbaPresentMi"
		mockCode := 200
		expected := "3th plural - pres - ind - act"
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		codes := []int{
			200,
		}

		meroi := []models.Meros{
			{
				Greek:   "ἀγαπάω",
				English: "to treat with affection, to caress, love, be fond of",
			},
		}

		jsonString, err := json.Marshal(meroi)
		assert.Nil(t, err)

		responses := []string{string(jsonString)}

		testClient, err := service.NewFakeClient(config, codes, responses)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
			Client:           testClient,
		}
		router := InitRoutes(&testConfig)
		response := performGetRequest(router, fmt.Sprintf("/dionysios/v1/checkGrammar?word=%s", searchWord))

		var declensions models.DeclensionTranslationResults
		err = json.NewDecoder(response.Body).Decode(&declensions)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		found := false
		for _, declension := range declensions.Results {
			if declension.Rule == expected {
				found = true
				break
			}

		}
		assert.True(t, found)
	})

	t.Run("HappyPathPresentVerbaThirdMi", func(t *testing.T) {
		searchWord := "δῐ́δωσῐ"
		fixtureFile := "dionysosVerbaPresentMi"
		mockCode := 200
		expected := "3th sing - pres - ind - act"
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		codes := []int{
			200,
		}

		meroi := []models.Meros{
			{
				Greek:   "δίδωμι",
				English: "to offer",
			},
		}

		jsonString, err := json.Marshal(meroi)
		assert.Nil(t, err)

		responses := []string{string(jsonString)}

		testClient, err := service.NewFakeClient(config, codes, responses)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
			Client:           testClient,
		}
		router := InitRoutes(&testConfig)
		response := performGetRequest(router, fmt.Sprintf("/dionysios/v1/checkGrammar?word=%s", searchWord))

		var declensions models.DeclensionTranslationResults
		err = json.NewDecoder(response.Body).Decode(&declensions)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.True(t, len(declensions.Results) == 1)
		assert.Equal(t, expected, declensions.Results[0].Rule)
	})

	t.Run("HappyPathPresentVerbaSecondPluralMai", func(t *testing.T) {
		searchWord := "μάχεσθε"
		fixtureFile := "dionysosVerbaPresentMai"
		mockCode := 200
		expected := "2nd plural - pres - mid - ind"
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		codes := []int{
			200,
		}

		meroi := []models.Meros{
			{
				Greek:   "μάχομαι",
				English: "to make war",
			},
		}

		jsonString, err := json.Marshal(meroi)
		assert.Nil(t, err)

		responses := []string{string(jsonString)}

		testClient, err := service.NewFakeClient(config, codes, responses)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
			Client:           testClient,
		}
		router := InitRoutes(&testConfig)
		response := performGetRequest(router, fmt.Sprintf("/dionysios/v1/checkGrammar?word=%s", searchWord))

		var declensions models.DeclensionTranslationResults
		err = json.NewDecoder(response.Body).Decode(&declensions)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, expected, declensions.Results[0].Rule)
	})

	t.Run("HappyPathPresentVerbaSecondSingMai", func(t *testing.T) {
		searchWord := "μάχει"
		fixtureFile := "dionysosVerbaPresentMai"
		mockCode := 200
		expected := "2nd sing - pres - mid - ind"
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)
		inMemoryCache, err := archytas.NewInMemoryBadgerClient()
		assert.Nil(t, err)

		declensionConfig, _ := QueryRuleSet(nil, "dionysios")
		assert.Nil(t, err)

		codes := []int{
			200,
		}

		meroi := []models.Meros{
			{
				Greek:   "μάχομαι",
				English: "to make war",
			},
		}

		jsonString, err := json.Marshal(meroi)
		assert.Nil(t, err)

		responses := []string{string(jsonString)}

		testClient, err := service.NewFakeClient(config, codes, responses)
		assert.Nil(t, err)

		testConfig := DionysosHandler{
			Elastic:          mockElasticClient,
			Cache:            inMemoryCache,
			Index:            elasticIndexDefault,
			DeclensionConfig: *declensionConfig,
			Client:           testClient,
		}
		router := InitRoutes(&testConfig)
		response := performGetRequest(router, fmt.Sprintf("/dionysios/v1/checkGrammar?word=%s", searchWord))

		var declensions models.DeclensionTranslationResults
		err = json.NewDecoder(response.Body).Decode(&declensions)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		ruleFound := false
		for _, res := range declensions.Results {
			if res.Rule == expected {
				ruleFound = true
			}
		}
		assert.True(t, ruleFound)
	})
}

func performGetRequest(r http.Handler, path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
