package migrate

import (
	"encoding/json"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// ToJSON serializes a MigrationPlan to JSON.
func (p *MigrationPlan) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// PlanFromJSON deserializes a MigrationPlan from JSON.
func PlanFromJSON(data []byte) (*MigrationPlan, error) {
	var plan MigrationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	// Initialize maps if nil
	if plan.Schema.Tables == nil {
		plan.Schema.Tables = make(map[string]ddl.Table)
	}
	return &plan, nil
}

// NewPlan creates a new empty MigrationPlan.
func NewPlan() *MigrationPlan {
	return &MigrationPlan{
		Schema: Schema{
			Tables: make(map[string]ddl.Table),
		},
		Migrations: []Migration{},
	}
}
