// JSON schema for the `handshake`

package signaling

import "github.com/pion/webrtc/v4"

type Signaling struct {
	SessionID string                   `json:"session_id"`
	Type      string                   `json:"type"`
	SDP       string                   `json:"sdp,omitempty"` // offer, answer, disconnect, candidate // the offer that the client makes to the server
	Candidate *webrtc.ICECandidateInit `json:"candidate,omitempty"`
}
