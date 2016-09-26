package engine

import (
	"github.com/juju/errors"
	"strings"
)

type FlowGenerator struct {
	flowKeyCreator *FlowKeyCreator
}

func NewFlowGenerator(c *FlowKeyCreator) *FlowGenerator {
	return &FlowGenerator{
		flowKeyCreator: c,
	}
}

func (g *FlowGenerator) GenerateFlow(flowConfig *FlowConfig, parentFlowKey []string, appName string) (*Flow, error) {
	flowKeyComponents := append(parentFlowKey, flowConfig.Name)
	flowKeyStr := strings.Join(flowKeyComponents, ".")
	flowKey := g.flowKeyCreator.CreateFlowKey(flowKeyStr)
	flow := &Flow{
		Key:         flowKey,
		ProjectName: appName,
		//Command:     cmd,
		FlowConfig: *flowConfig,
	}

	flows := []*Flow{}

	for _, c := range flow.FlowConfigs {
		f, err := g.GenerateFlow(c, flowKeyComponents, appName)

		if err != nil {
			return nil, errors.Trace(err)
		}

		flows = append(flows, f)
	}

	flow.Flows = flows

	return flow, nil
}
