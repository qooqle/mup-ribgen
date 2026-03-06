package mode2

import (
	"context"

	"google.golang.org/grpc"
)

// SMFPluginServiceServer is implemented by Receiver (server side).
type SMFPluginServiceServer interface {
	ReportSession(context.Context, *SessionEvent) (*EventResponse, error)
}

// RegisterSMFPluginServiceServer registers the SMFPluginService with a gRPC Server.
func RegisterSMFPluginServiceServer(s *grpc.Server, srv SMFPluginServiceServer) {
	s.RegisterService(&smfPluginServiceDesc, srv)
}

var smfPluginServiceDesc = grpc.ServiceDesc{
	ServiceName: "mode2.SMFPluginService",
	HandlerType: (*SMFPluginServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ReportSession",
			Handler:    reportSessionHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mode2/mode2.proto",
}

func reportSessionHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SessionEvent)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SMFPluginServiceServer).ReportSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mode2.SMFPluginService/ReportSession",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SMFPluginServiceServer).ReportSession(ctx, req.(*SessionEvent))
	}
	return interceptor(ctx, in, info, handler)
}
