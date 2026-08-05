package dionysios_test

import (
	"context"
	"fmt"
	"time"

	dionysiosv1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const herodotusReferenceText = "μήτε ἔργα μεγάλα τε καὶ θωμαστά, τὰ μὲν Ἕλλησι τὰ δὲ βαρβάροισι ἀποδεχθέντα, ἀκλεᾶ γένηται,"

var _ = Describe("Dionysios gRPC endpoints", func() {
	It("reports a useful health baseline", func() {
		ctx, cancel := callContext()
		defer cancel()

		response, err := dionysiosClient.Health(ctx, &dionysiosv1.HealthRequest{})

		Expect(err).NotTo(HaveOccurred())
		Expect(response.GetHealthy()).To(BeTrue())
		Expect(response.GetTime()).NotTo(BeEmpty())
		Expect(response.GetMapping()).NotTo(BeNil())
		Expect(response.GetMapping().GetLoaded()).To(BeTrue())
		Expect(response.GetMapping().GetRuleCount()).To(BeNumerically(">", 0))
	})

	DescribeTable("rejects incomplete grammar requests",
		func(word string) {
			ctx, cancel := callContext()
			defer cancel()

			_, err := dionysiosClient.CheckGrammar(ctx, &dionysiosv1.CheckGrammarRequest{Word: word})

			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		},
		Entry("empty", ""),
		Entry("whitespace", "   "),
	)

	It("analyzes a representative word and includes its audit", func() {
		ctx, cancel := callContext()
		defer cancel()

		response, err := dionysiosClient.CheckGrammar(ctx, &dionysiosv1.CheckGrammarRequest{
			Word:         "ὁ",
			IncludeAudit: true,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(response.GetResults()).NotTo(BeEmpty())
		Expect(response.GetAudit()).NotTo(BeNil())
		Expect(response.GetAudit().GetWord()).To(Equal("ὁ"))
		Expect(response.GetAudit().GetEvents()).NotTo(BeEmpty())
	})

	It("validates research before calling downstream services", func() {
		ctx, cancel := callContext()
		defer cancel()

		_, err := dionysiosClient.Research(ctx, &dionysiosv1.ResearchRequest{Rootword: "  "})

		Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
	})

	DescribeTable("validates text-mode identity and content",
		func(request *dionysiosv1.TextModeRequest, message string) {
			ctx, cancel := callContext()
			defer cancel()

			_, err := dionysiosClient.TextMode(ctx, request)

			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			Expect(status.Convert(err).Message()).To(ContainSubstring(message))
		},
		Entry("missing text", &dionysiosv1.TextModeRequest{SessionId: "galenos"}, "text is required"),
		Entry("missing session", &dionysiosv1.TextModeRequest{Text: "ὁ"}, "session_id is required"),
		Entry("punctuation only", &dionysiosv1.TextModeRequest{Text: "...", SessionId: "galenos"}, "at least one word"),
	)

	It("produces structured evidence for the Herodotus reference passage", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ctx = metadata.AppendToOutgoingContext(ctx, "x-forwarded-for", "127.0.0.99")

		response, err := dionysiosClient.TextMode(ctx, &dionysiosv1.TextModeRequest{
			Text:         herodotusReferenceText,
			SessionId:    fmt.Sprintf("galenos-herodotus-%d", time.Now().UnixNano()),
			IncludeAudit: true,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(response.GetOriginalText()).To(Equal(herodotusReferenceText))
		Expect(response.GetTokens()).To(HaveLen(15))
		Expect(response.GetLiteralTranslation()).NotTo(BeEmpty())
		Expect(resolvedTokenCount(response.GetTokens())).To(BeNumerically(">=", 12))

		assertSingleTokenResult(response.GetTokens(), "μεγάλα", "μέγας", "adjective - plural - neut - nom/voc/acc")
		assertSingleTokenResult(response.GetTokens(), "θωμαστά", "θαυμαστός", "adjective - plural - neut - nom/voc/acc (ionic)")
		assertSingleTokenResult(response.GetTokens(), "γένηται", "γίγνομαι", "3rd sing - aor - subj - mid/pas")

		search := response.GetTextSearch()
		Expect(search).NotTo(BeNil())
		Expect(search.GetSearched()).To(BeTrue())
		Expect(search.GetFound()).To(BeTrue())
		Expect(search.GetStatus()).To(Equal("found"))
		Expect(search.GetMatchCount()).To(BeNumerically(">=", 1))
		Expect(search.GetMessage()).To(ContainSubstring("match found using"))
		Expect(search.GetMatches()).To(ContainElement(And(
			HaveField("Author", "Herodotus"),
			HaveField("Book", "Histories"),
			HaveField("Reference", "1.1"),
		)))

		Expect(response.GetAudit()).NotTo(BeNil())
		Expect(response.GetAudit().GetEvents()).To(ContainElement(And(
			HaveField("Step", "text.search"),
			HaveField("Status", "found"),
			HaveField("Source", "kallimachos"),
		)))
	})
})

func assertSingleTokenResult(tokens []*dionysiosv1.TextToken, token, rootWord, rule string) {
	var found *dionysiosv1.TextToken
	for _, candidate := range tokens {
		if candidate.GetToken() == token {
			found = candidate
			break
		}
	}
	Expect(found).NotTo(BeNil(), "expected token %q", token)
	Expect(found.GetResolved()).To(BeTrue())
	Expect(found.GetResults()).To(HaveLen(1), "expected one canonical lexical result for %q", token)
	Expect(found.GetResults()[0].GetRootWord()).To(Equal(rootWord))
	Expect(found.GetResults()[0].GetRule()).To(Equal(rule))
}

func resolvedTokenCount(tokens []*dionysiosv1.TextToken) int {
	count := 0
	for _, token := range tokens {
		if token.GetResolved() {
			count++
		}
	}
	return count
}
