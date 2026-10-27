package grpc

import pb "github.com/Will413028/hawkr/go-pkg/proto/im"

func messageTypeToString(t pb.MessageType) string {
	switch t {
	case pb.MessageType_MESSAGE_TYPE_TEXT:
		return "text"
	case pb.MessageType_MESSAGE_TYPE_IMAGE:
		return "image"
	default:
		return "message"
	}
}
