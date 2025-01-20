package drp

// when join a user or a node, the user's all nodes key will add to the drp node, so the drp node can forward the packet to the dst node
// and also check whether the user or node is valid.
type ServerInfo struct {
	// nothing, just send server header
}
