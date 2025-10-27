package resource

import (
	"context"

	"github.com/wireflowio/wireflow-controller/pkg/controller"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type EventHandler interface {
	RunWorker(ctx context.Context)
	ProcessNextItem(ctx context.Context) bool
	EventType() EventType
	Informer() cache.SharedIndexInformer

	WorkQueue() workqueue.TypedRateLimitingInterface[controller.WorkerItem]
}

type EventType int

const (
	NodeType EventType = iota
	NetworkType
	PolicyType
	RuleType
)

func (e EventType) String() string {
	switch e {
	case NodeType:
		return "node"
	case NetworkType:
		return "network"
	case PolicyType:
		return "policy"
	case RuleType:
		return "rule"
	default:
	}
	return ""
}
