package ai

import (
	"sync"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// ToolRegistry manages a collection of AI tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools []ai.ToolRef
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool ai.ToolRef) {
	r.mu.Lock()
	r.tools = append(r.tools, tool)
	r.mu.Unlock()
}

// AllTools returns all registered tools.
func (r *ToolRegistry) AllTools() []ai.ToolRef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools
}

// RegisterTool creates and registers a new tool with the given parameters.
func RegisterTool[In, Out any](r *ToolRegistry, g *genkit.Genkit, name, description string, fn ai.ToolFunc[In, Out]) *ai.ToolDef[In, Out] {
	tool := genkit.DefineTool(g, name, description, fn)
	r.Register(tool)
	return tool
}
