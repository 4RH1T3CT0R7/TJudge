package game

import (
	"fmt"
	"sort"
)

// Registry — реестр игровых плагинов
// не потокобезопасен: все Register должны отработать до первого Get/List/Has
// (обычно на старте, при инициализации)
type Registry struct {
	plugins map[string]*GamePlugin
}

// NewRegistry — пустой реестр
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]*GamePlugin),
	}
}

// Register добавляет плагин, ошибка если имя уже занято
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

// Get — плагин по имени, плюс флаг нашли/нет
func (r *Registry) Get(name string) (*GamePlugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// List отдаёт все плагины, отсортированы по имени
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

// Has — есть ли плагин с таким именем
func (r *Registry) Has(name string) bool {
	_, ok := r.plugins[name]
	return ok
}
