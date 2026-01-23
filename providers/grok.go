package providers

import (
	"os"
)

var OpenaiProviderGrok OpenaiProvider = OpenaiProvider {
	Endpoint: "https://api.x.ai/v1/chat/completions",
	Models: NewModelSelector("grok-4.1-non-reasoning", "grok-4.1-non-reasoning", "grok-4.1"),
	ApiKey: os.Getenv("XAI_API_KEY"),
	WebSearchField: map[string]any {
		"search_parameters": map[string]any{},
	},
	UseDeveloperRole: false,
}
