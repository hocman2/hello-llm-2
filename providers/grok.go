package providers

import (
	"os"
)

var OpenaiProviderGrok OpenaiProvider = OpenaiProvider {
	Endpoint: "https://api.x.ai/v1/chat/completions",
	Model: "grok-4-1-fast-non-reasoning",
	ApiKey: os.Getenv("XAI_API_KEY"),
	ModelWebSearch: "grok-4-1-fast-non-reasoning",
	WebSearchField: map[string]any {
		"search_parameters": map[string]any{},
	},
	UseDeveloperRole: false,
}
