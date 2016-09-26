package engine

import (
	"github.com/juju/errors"
)

type FlowRegistry struct {
	flows map[string]*Flow
}

func NewFlowRegistry() *FlowRegistry {
	return &FlowRegistry{
		flows: map[string]*Flow{},
	}
}

func (p *FlowRegistry) Flows() map[string]*Flow {
	return p.flows
}

func (p *FlowRegistry) FindFlow(flowKey FlowKey) (*Flow, error) {
	t := p.flows[flowKey.String()]

	if t == nil {
		return nil, errors.Errorf("No Flow exists for the flow key `%s`", flowKey.String())
	}

	return t, nil
}

func (p *FlowRegistry) RegisterFlow(flowKey FlowKey, flowDef *Flow) {
	p.flows[flowKey.String()] = flowDef
}

func (p *FlowRegistry) RegisterFlows(flow *Flow) {
	p.RegisterFlow(flow.Key, flow)

	for _, child := range flow.Flows {
		p.RegisterFlows(child)
	}
}

func (p *FlowRegistry) AllFlowKeys() []string {
	allFlows := []string{}
	for _, t := range p.flows {
		allFlows = append(allFlows, t.Key.String())
	}
	return allFlows
}
