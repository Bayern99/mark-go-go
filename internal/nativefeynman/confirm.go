package nativefeynman

import (
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

func (p *panel) confirmationReturn(key string) *ui.Handle {
	if p.confirmationReturns == nil {
		p.confirmationReturns = make(map[string]*ui.Handle)
	}
	if p.confirmationReturns[key] == nil {
		p.confirmationReturns[key] = new(ui.Handle)
	}
	return p.confirmationReturns[key]
}

func (p *panel) confirmOnClick(key string, trigger ui.Element, spec portrait.ConfirmSpec) {
	returnFocus := p.confirmationReturn(key)
	trigger.Bind(returnFocus)
	if !trigger.Clicked() {
		return
	}
	spec.Return = returnFocus
	p.confirmationSpec = spec
	p.confirmationOpen = true
}

func (p *panel) drawConfirmation(c *ui.Context) {
	portrait.Confirm(c, &p.confirmationOpen, p.confirmationSpec)
}
