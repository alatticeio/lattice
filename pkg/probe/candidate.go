package probe

import (
	"github.com/linkanyio/ice"
	"linkany/internal"
)

func (p *prober) GetCandidates(agent *internal.Agent) string {
	var (
		err        error
		candidates []ice.Candidate
		candString string
	)
	select {
	case <-p.gatherCh:
		candidates, err = agent.GetLocalCandidates()
		if err != nil {
			return ""
		}
		for i, candidate := range candidates {
			candString = candidate.Marshal()
			if i != len(candidates)-1 {
				candString += ";"
			}
		}
		p.logger.Verbosef("gathered candidates >>>: %v", candString)
		return candString
	}
}
