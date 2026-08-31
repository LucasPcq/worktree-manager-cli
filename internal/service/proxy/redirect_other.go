//go:build !darwin

package proxy

import "github.com/LucasPcq/wtm/internal/domain"

type unsupported struct{}

func NewRedirector(_ RedirectorParams) Redirector { return unsupported{} }

func (unsupported) Plan(PlanParams) (Plan, error) { return Plan{}, domain.ErrProxyRedirectUnsupported }
func (unsupported) Apply(PlanParams) error        { return domain.ErrProxyRedirectUnsupported }
func (unsupported) Remove() error                 { return domain.ErrProxyRedirectUnsupported }
func (unsupported) Inspect() domain.ProxyStatus   { return domain.ProxyStatus{} }
