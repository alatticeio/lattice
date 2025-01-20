package wrapper

import "linkany/pkg/drp"

// TODO: macOS, FreeBSD and other BSDs likely do support this feature set, but
// use alternatively named flags and need ports and require testing.

// getSrcFromControl parses the control for PKTINFO and if found updates ep with
// the source information found.
func getSrcFromControl(control []byte, ep *drp.AnyEndpoint) {
}

// setSrcControl parses the control for PKTINFO and if found updates ep with
// the source information found.
func setSrcControl(control *[]byte, ep *drp.AnyEndpoint) {
}

// srcControlSize returns the recommended buffer size for pooling sticky control
// data.
const srcControlSize = 0

const StdNetSupportsStickySockets = false
