package engine

import (
	"github.com/juju/errors"
	"github.com/mumoshu/variant/api/step"
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

func (p *FlowRegistry) FindFlow(flowKey step.Key) (*Flow, error) {
	t := p.flows[flowKey.ShortString()]

	if t == nil {
		return nil, errors.Errorf("No Flow exists for the flow key `%s`", flowKey.ShortString())
	}

	return t, nil
}

func (p *FlowRegistry) RegisterFlow(flowKey step.Key, flowDef *Flow) {
	p.flows[flowKey.ShortString()] = flowDef
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
