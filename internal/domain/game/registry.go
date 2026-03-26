package game

import (
	"fmt"
	"sort"
)

// Registry holds all registered game plugins.
// It is NOT safe for concurrent use. All Register calls must complete
// before any Get/List/Has calls (typically during initialization).
type Registry struct {
	plugins map[string]*GamePlugin
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*GamePlugin),
	}
}

// Register adds a game plugin to the registry.
// Returns an error if a plugin with the same name is already registered.
func (r *Registry) Register(plugin *GamePlugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin must not be nil")
	}
	if plugin.Name == "" {
		return fmt.Errorf("plugin name must not be empty")
	}
	if _, exists := r.plugins[plugin.Name]; exists {
		return fmt.Errorf("game plugin %q is already registered", plugin.Name)
	}
	r.plugins[plugin.Name] = plugin
	return nil
}

// Get returns the plugin with the given name and a boolean indicating whether it was found.
func (r *Registry) Get(name string) (*GamePlugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all registered plugins sorted alphabetically by name.
func (r *Registry) List() []*GamePlugin {
	result := make([]*GamePlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Has returns true if a plugin with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.plugins[name]
	return ok
}
