// Package ai estimates the calories of freely described or photographed food.
//
// The Claude API is used when an API key is configured; otherwise the app falls
// back to a local lookup table of common foods, so calorie estimation keeps
// working with no internet connection and no external account.
package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// DefaultModel is the model used unless the add-on configuration overrides it.
const DefaultModel = "claude-opus-5"

// ErrNoKey is returned when an AI estimate is requested without an API key.
var ErrNoKey = errors.New("kein Anthropic API-Key konfiguriert")

// Item is one recognised food component.
type Item struct {
	Name     string  `json:"name"`
	Amount   string  `json:"amount"`
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
}

// Estimate is the full result handed back to the user for confirmation.
type Estimate struct {
	Items      []Item  `json:"items"`
	Kcal       float64 `json:"kcal"`
	ProteinG   float64 `json:"protein_g"`
	CarbsG     float64 `json:"carbs_g"`
	FatG       float64 `json:"fat_g"`
	Confidence string  `json:"confidence"` // high | medium | low
	// Assumptions explains what the estimate had to guess (portion size, added
	// fat, ...) so the user can correct it before saving.
	Assumptions string `json:"assumptions"`
	MealType    string `json:"meal_type"`
	Source      string `json:"source"` // "ai_text", "ai_image" or "local_db"
	Model       string `json:"model,omitempty"`
}

// Client talks to the Claude API.
type Client struct {
	api     anthropic.Client
	model   string
	enabled bool
}

// New builds a client. An empty apiKey yields a disabled client that still
// answers via the local food table.
func New(apiKey, model string) *Client {
	c := &Client{model: strings.TrimSpace(model)}
	if c.model == "" {
		c.model = DefaultModel
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		c.api = anthropic.NewClient(option.WithAPIKey(key))
		c.enabled = true
	}
	return c
}

// Enabled reports whether AI estimation is available.
func (c *Client) Enabled() bool { return c.enabled }

// Model returns the configured model id.
func (c *Client) Model() string { return c.model }

// estimateTool is the structured shape Claude must fill in. Forcing a tool call
// is what guarantees parseable output instead of prose about calories.
var estimateTool = anthropic.ToolParam{
	Name:        "record_estimate",
	Description: anthropic.String("Record the nutritional estimate for the food described or shown."),
	InputSchema: anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"items": map[string]any{
				"type":        "array",
				"description": "One entry per distinct food component.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "Short name of the food."},
						"amount":    map[string]any{"type": "string", "description": "Portion, e.g. '2 Kugeln', '150 g', '1 Scheibe'."},
						"kcal":      map[string]any{"type": "number"},
						"protein_g": map[string]any{"type": "number"},
						"carbs_g":   map[string]any{"type": "number"},
						"fat_g":     map[string]any{"type": "number"},
					},
					"required": []string{"name", "amount", "kcal", "protein_g", "carbs_g", "fat_g"},
				},
			},
			"confidence": map[string]any{
				"type":        "string",
				"enum":        []string{"high", "medium", "low"},
				"description": "How sure you are. Use 'low' when the portion size is genuinely unclear.",
			},
			"assumptions": map[string]any{
				"type":        "string",
				"description": "One short sentence naming what you had to assume, in the requested language.",
			},
			"meal_type": map[string]any{
				"type": "string",
				"enum": []string{"breakfast", "lunch", "dinner", "snack"},
			},
		},
		Required: []string{"items", "confidence", "assumptions", "meal_type"},
	},
}

// toolInput mirrors the schema above.
type toolInput struct {
	Items       []Item `json:"items"`
	Confidence  string `json:"confidence"`
	Assumptions string `json:"assumptions"`
	MealType    string `json:"meal_type"`
}

func systemPrompt(lang string) string {
	langLine := "Answer in German. Names and the assumptions sentence must be in German."
	if lang == "en" {
		langLine = "Answer in English. Names and the assumptions sentence must be in English."
	}
	return strings.Join([]string{
		"You estimate the nutritional content of everyday food for a home fitness tracker.",
		"Give realistic household portions: a scoop of ice cream, a slice of bread, a normal plate.",
		"Do not round to suspiciously neat numbers, and do not pad estimates 'to be safe' — an honest",
		"mid-range guess is more useful than a high one, because the user compares it against a daily target.",
		"If a portion size is not visible or stated, assume a typical adult serving and say so in the assumptions.",
		"Break a plate into its components rather than reporting one lump sum.",
		langLine,
		"Always call the record_estimate tool. Never answer in prose.",
	}, " ")
}

// EstimateText estimates the nutrition of a free-text description.
func (c *Client) EstimateText(ctx context.Context, description, lang string) (Estimate, error) {
	if !c.enabled {
		return Estimate{}, ErrNoKey
	}
	prompt := "Estimate the nutrition of this food: " + description
	return c.call(ctx, lang, "ai_text", anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)))
}

// EstimateImage estimates the nutrition of a photographed meal. imageData is
// the raw image bytes; mediaType is its MIME type.
func (c *Client) EstimateImage(ctx context.Context, imageData []byte, mediaType, note, lang string) (Estimate, error) {
	if !c.enabled {
		return Estimate{}, ErrNoKey
	}
	prompt := "Estimate the nutrition of the food in this photo. Identify each component you can see."
	if strings.TrimSpace(note) != "" {
		prompt += " Additional context from the user: " + note
	}
	b64 := base64.StdEncoding.EncodeToString(imageData)
	msg := anthropic.NewUserMessage(
		anthropic.NewImageBlockBase64(mediaType, b64),
		anthropic.NewTextBlock(prompt),
	)
	return c.call(ctx, lang, "ai_image", msg)
}

// call issues the request and unpacks the forced tool call.
func (c *Client) call(ctx context.Context, lang, source string, msg anthropic.MessageParam) (Estimate, error) {
	resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{{
			Text: systemPrompt(lang),
			// The system prompt and tool schema are identical on every call,
			// so caching them keeps repeat estimates cheap.
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		// This is a narrow perception task, not a reasoning problem — low
		// effort keeps a photo estimate fast and cheap.
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortLow},
		Tools:        []anthropic.ToolUnionParam{{OfTool: &estimateTool}},
		ToolChoice:   anthropic.ToolChoiceParamOfTool(estimateTool.Name),
		Messages:     []anthropic.MessageParam{msg},
	})
	if err != nil {
		return Estimate{}, fmt.Errorf("claude request: %w", err)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return Estimate{}, fmt.Errorf("claude declined this request (%s)", resp.StopDetails.Category)
	}

	for _, block := range resp.Content {
		use, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok || use.Name != estimateTool.Name {
			continue
		}
		var in toolInput
		if err := json.Unmarshal([]byte(use.JSON.Input.Raw()), &in); err != nil {
			return Estimate{}, fmt.Errorf("parse estimate: %w", err)
		}
		return finalize(in, source, c.model), nil
	}
	return Estimate{}, errors.New("claude returned no estimate")
}

// finalize sums the items and normalises the result.
func finalize(in toolInput, source, model string) Estimate {
	e := Estimate{
		Items:       in.Items,
		Confidence:  in.Confidence,
		Assumptions: in.Assumptions,
		MealType:    in.MealType,
		Source:      source,
		Model:       model,
	}
	for _, it := range in.Items {
		e.Kcal += it.Kcal
		e.ProteinG += it.ProteinG
		e.CarbsG += it.CarbsG
		e.FatG += it.FatG
	}
	e.Kcal = round(e.Kcal)
	e.ProteinG = round(e.ProteinG)
	e.CarbsG = round(e.CarbsG)
	e.FatG = round(e.FatG)
	if e.Confidence == "" {
		e.Confidence = "medium"
	}
	if e.MealType == "" {
		e.MealType = "snack"
	}
	return e
}

func round(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
