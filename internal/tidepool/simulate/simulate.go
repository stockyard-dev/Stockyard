// Package simulate provides what-if scenario simulation.
package simulate

// Scenario defines a simulation scenario.
type Scenario struct {
	Variable string  `json:"variable"`
	Change   float64 `json:"change"`
}

// Result holds simulation output.
type Result struct {
	Predicted map[string]float64 `json:"predicted"`
}

// Run executes a simple linear simulation.
func Run(baseline map[string]float64, scenario Scenario) Result {
	predicted := make(map[string]float64)
	for k, v := range baseline {
		predicted[k] = v * (1 + scenario.Change)
	}
	return Result{Predicted: predicted}
}
