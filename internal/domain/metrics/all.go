package metrics

import "github.com/jedi-knights/kyber/internal/domain"

// DefaultRegistry returns a Registry pre-populated with every metric shipped
// with kyber. Adding a new metric means: implement domain.Metric, register
// here, write tests. Nothing else changes.
func DefaultRegistry() *domain.Registry {
	r := domain.NewRegistry()
	r.MustRegister(NewCyclomatic())
	r.MustRegister(NewCognitive())
	r.MustRegister(NewDifficulty())
	r.MustRegister(NewEffort())
	r.MustRegister(NewFuncLen())
	r.MustRegister(NewHalstead())
	r.MustRegister(NewMaintainability())
	r.MustRegister(NewNesting())
	r.MustRegister(NewNPath())
	r.MustRegister(NewReadability())
	r.MustRegister(NewReturns())
	r.MustRegister(NewTestability())
	return r
}
