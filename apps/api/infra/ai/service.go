package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/redis/go-redis/v9"
)

// Service provides AI generation capabilities with usage tracking.
type Service struct {
	g             *genkit.Genkit
	tracker       *UsageTracker
	reserveTokens int64
}

// NewService creates a new AI service with the given configuration.
func NewService(ctx context.Context, cfg Config, rdb *redis.Client) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	g := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.GoogleAI{}),
		genkit.WithDefaultModel("googleai/"+cfg.DefaultModel),
	)

	return &Service{
		g:             g,
		tracker:       NewUsageTracker(rdb, cfg.DefaultQuota),
		reserveTokens: cfg.ReserveTokens,
	}, nil
}

// GenerateTextRequest contains the parameters for text generation.
type GenerateTextRequest struct {
	UserID string
	Quota  int64  // Optional: custom quota (0 = use default)
	Model  string // Optional: model override
	System string // System prompt
	Prompt string // User prompt
}

// GenerateTextResponse contains the result of text generation.
type GenerateTextResponse struct {
	Text       string
	TokensUsed int64
}

// GenerateText generates text based on the given prompt with usage tracking.
func (s *Service) GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error) {
	key, _, err := s.tracker.Reserve(ctx, req.UserID, s.reserveTokens, req.Quota)
	if err != nil {
		return nil, err
	}

	opts := []ai.GenerateOption{ai.WithPrompt(req.Prompt)}
	if req.System != "" {
		opts = append(opts, ai.WithSystem(req.System))
	}
	if req.Model != "" {
		opts = append(opts, ai.WithModelName("googleai/"+req.Model))
	}

	resp, err := genkit.Generate(ctx, s.g, opts...)
	tokensUsed := s.finalizeUsage(ctx, key, resp, err)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGenerationFailed, err)
	}

	return &GenerateTextResponse{Text: resp.Text(), TokensUsed: tokensUsed}, nil
}

// GenerateWithTools generates text with tool calling support.
func (s *Service) GenerateWithTools(ctx context.Context, req GenerateTextRequest, tools []ai.ToolRef) (*GenerateTextResponse, error) {
	key, _, err := s.tracker.Reserve(ctx, req.UserID, s.reserveTokens, req.Quota)
	if err != nil {
		return nil, err
	}

	opts := []ai.GenerateOption{ai.WithPrompt(req.Prompt)}
	if req.System != "" {
		opts = append(opts, ai.WithSystem(req.System))
	}
	if req.Model != "" {
		opts = append(opts, ai.WithModelName("googleai/"+req.Model))
	}
	if len(tools) > 0 {
		opts = append(opts, ai.WithTools(tools...))
	}

	resp, err := genkit.Generate(ctx, s.g, opts...)
	tokensUsed := s.finalizeUsage(ctx, key, resp, err)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGenerationFailed, err)
	}

	return &GenerateTextResponse{Text: resp.Text(), TokensUsed: tokensUsed}, nil
}

// GenerateData generates structured data of type T from a prompt.
func GenerateData[T any](ctx context.Context, s *Service, userID string, quota int64, model string, prompt string) (*T, int64, error) {
	key, _, err := s.tracker.Reserve(ctx, userID, s.reserveTokens, quota)
	if err != nil {
		return nil, 0, err
	}

	opts := []ai.GenerateOption{
		ai.WithPrompt(prompt),
		ai.WithOutputFormat(ai.OutputFormatJSON),
	}
	if model != "" {
		opts = append(opts, ai.WithModelName("googleai/"+model))
	}

	resp, err := genkit.Generate(ctx, s.g, opts...)
	tokensUsed := s.finalizeUsage(ctx, key, resp, err)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrGenerationFailed, err)
	}

	var result T
	if err := json.Unmarshal([]byte(resp.Text()), &result); err != nil {
		return nil, tokensUsed, fmt.Errorf("parse response: %w", err)
	}

	return &result, tokensUsed, nil
}

// finalizeUsage adjusts the reserved tokens to match actual usage.
// On error, refunds all reserved tokens. On success, charges/refunds the difference.
func (s *Service) finalizeUsage(ctx context.Context, key string, resp *ai.ModelResponse, genErr error) int64 {
	if genErr != nil {
		_ = s.tracker.Adjust(ctx, key, -s.reserveTokens)
		return 0
	}

	var tokensUsed int64
	if resp != nil && resp.Usage != nil {
		tokensUsed = int64(resp.Usage.TotalTokens)
	}

	// Adjust: negative = refund unused, positive = charge extra
	diff := tokensUsed - s.reserveTokens
	if diff != 0 {
		_ = s.tracker.Adjust(ctx, key, diff)
	}

	return tokensUsed
}

// Genkit returns the underlying Genkit instance for advanced usage.
func (s *Service) Genkit() *genkit.Genkit {
	return s.g
}

// Tracker returns the usage tracker.
func (s *Service) Tracker() *UsageTracker {
	return s.tracker
}
