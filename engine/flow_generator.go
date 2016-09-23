package engine

import (
	"github.com/juju/errors"
	"strings"
)

type FlowGenerator struct {
}

func (g *FlowGenerator) GenerateFlow(flowConfig *FlowConfig, parentFlowKey []string, p *Application) (*Flow, error) {
	flowKeyComponents := append(parentFlowKey, flowConfig.Name)
	flowKeyStr := strings.Join(flowKeyComponents, ".")
	flowKey := p.CreateFlowKey(flowKeyStr)
	flow := &Flow{
		Key:         flowKey,
		ProjectName: p.Name,
		//Command:     cmd,
		FlowConfig: *flowConfig,
	}

	flows := []*Flow{}

	for _, c := range flow.FlowConfigs {
		f, err := g.GenerateFlow(c, flowKeyComponents, p)

		if err != nil {
			return nil, errors.Trace(err)
		}

		flows = append(flows, f)
	}

	flow.Flows = flows

	return flow, nil
}
