// Package uuid provides UUID generation for k6 using google/uuid
package uuid

import (
	"github.com/google/uuid"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/uuid", new(RootModule))
}

// RootModule is the global module instance
type RootModule struct{}

// ModuleInstance is the per-VU module instance
type ModuleInstance struct {
	vu modules.VU
}

// NewModuleInstance creates a new instance for each VU
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu}
}

// Exports returns the exports of the module
func (m *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"v4":      m.V4,
			"v7":      m.V7,
			"newV4":   m.V4,
			"newV7":   m.V7,
			"uuidv4":  m.V4,
			"uuidv7":  m.V7,
		},
	}
}

// V4 generates a new UUIDv4
func (m *ModuleInstance) V4() string {
	return uuid.New().String()
}

// V7 generates a new UUIDv7 (time-ordered)
func (m *ModuleInstance) V7() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to V4 if V7 fails (should not happen)
		return uuid.New().String()
	}
	return id.String()
}
