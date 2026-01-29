package providers

import (
	"os"
)

var OpenaiProviderGrok OpenaiProvider = OpenaiProvider {
	Endpoint: "https://api.x.ai/v1/responses",
	Models: NewModelSelector("grok-4-1-fast-non-reasoning", "grok-4-1-fast-non-reasoning", "grok-4-1-fast-reasoning"),
	ApiKey: os.Getenv("XAI_API_KEY"),
	UseDeveloperRole: false,
}
