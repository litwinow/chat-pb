package protocol

const UsernameLength = 5
const MaxThrottle = 50
const MaxMessageLength = 500

const (
	InitReq = iota
	InitResp
	Message
)
