package dionysios_test

import (
	dionysiosv1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	DescribeTable("advertises scholar endpoints that are not implemented yet",
		func(call func() error) {
			Expect(status.Code(call())).To(Equal(codes.Unimplemented))
		},
		Entry("ExplainWord", func() error {
			ctx, cancel := callContext()
			defer cancel()
			_, err := dionysiosClient.ExplainWord(ctx, &dionysiosv1.ExplainWordRequest{Word: "λόγος", SessionId: "galenos"})
			return err
		}),
		Entry("DiveText", func() error {
			ctx, cancel := callContext()
			defer cancel()
			_, err := dionysiosClient.DiveText(ctx, &dionysiosv1.DiveTextRequest{TextId: "baseline", SessionId: "galenos"})
			return err
		}),
	)
})
