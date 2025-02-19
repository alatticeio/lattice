package probe

import (
	"github.com/linkanyio/ice"
	"k8s.io/klog/v2"
)

func GetCandidates(agent *ice.Agent, gatherCh chan interface{}) string {
	<-gatherCh
	var err error
	var ch = make(chan struct{})
	var candidates []ice.Candidate
	go func() {
		for {
			candidates, err = agent.GetLocalCandidates()
			if err != nil || len(candidates) == 0 {
				continue
			}

			close(ch)
			break
		}
	}()

	select {
	case <-ch:
	}

	var candString string
	for i, candidate := range candidates {
		candString = candidate.Marshal()
		if i != len(candidates)-1 {
			candString += ";"
		}
	}

	klog.Infof("gathered candidates >>>: %v", candString)
	return candString
}
