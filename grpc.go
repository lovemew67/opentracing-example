package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	UnimplementedEchoServiceServer
}

func (s *grpcServer) Echo(ctx context.Context, req *HiRequest) (res *HiResponse, err error) {
	fmt.Printf("received proto.HiRequest: %s \n", req.Message)

	// https://github.com/opentracing/opentracing-go/blob/master/gocontext.go

	var sp opentracing.Span
	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan != nil {
		sp = opentracing.StartSpan("echo handler", opentracing.ChildOf(parentSpan.Context()))
	} else {
		sp = opentracing.StartSpan("echo handler")
	}
	defer sp.Finish()

	sp.SetTag("req message", req.Message)

	res = &HiResponse{
		Message: fmt.Sprintf("reveivced: %s", req.Message),
	}
	return
}

func startUpGRPC() {
	go func() {
		time.Sleep(5 * time.Second)

		// Set up a connection to the server.
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := grpc.DialContext(ctx, "localhost:8080", grpc.WithInsecure(), grpc.WithBalancerName("round_robin"), grpc.WithUnaryInterceptor(ClientInterceptor(globalTracer)))
		if err != nil {
			log.Fatalf("did not connect: %+v", err)
		}
		defer conn.Close()
		grpcClient := NewEchoServiceClient(conn)

		// resp 1
		resp1, err := grpcClient.Echo(context.Background(), &HiRequest{
			Message: "aaa",
		})
		if err != nil {
			log.Fatalf("echo err: %+v", err)
		}
		log.Printf("resp: %+v \n", resp1)

		// resp 2
		resp2, err := grpcClient.Echo(context.Background(), &HiRequest{
			Message: "bbb",
		})
		if err != nil {
			log.Fatalf("echo err: %+v", err)
		}
		log.Printf("resp: %+v \n", resp2)
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(serverInterceptor(globalTracer)))
	RegisterEchoServiceServer(s, &grpcServer{})
	reflection.Register(s)
	fmt.Printf("start gRPC server: %d \n", *port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %+v", err)
	}
}

// https://www.selinux.tech/golang/grpc/grpc-tracing

type MDCarrier struct {
	metadata.MD
}

func (m MDCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, strs := range m.MD {
		for _, v := range strs {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m MDCarrier) Set(key, val string) {
	m.MD[key] = append(m.MD[key], val)
}

func ClientInterceptor(tracer opentracing.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var parentCtx opentracing.SpanContext
		parentSpan := opentracing.SpanFromContext(ctx)
		if parentSpan != nil {
			parentCtx = parentSpan.Context()
		}
		span := tracer.StartSpan(
			method,
			opentracing.ChildOf(parentCtx),
			opentracing.Tag{Key: string(ext.Component), Value: "gRPC Client"},
			ext.SpanKindRPCClient,
		)

		defer span.Finish()
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		err := tracer.Inject(
			span.Context(),
			opentracing.TextMap,
			MDCarrier{md},
		)
		if err != nil {
			fmt.Printf("inject span error :%+v \n", err.Error())
		}

		newCtx := metadata.NewOutgoingContext(ctx, md)
		err = invoker(newCtx, method, request, reply, cc, opts...)
		if err != nil {
			fmt.Printf("call error : %+v \n", err.Error())
		}
		return err
	}
}

func serverInterceptor(tracer opentracing.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if tracer == nil {
			grpclog.Errorf("nil tracer")
			return
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		spanContext, err := tracer.Extract(
			opentracing.TextMap,
			MDCarrier{
				md,
			},
		)
		if err != nil && err != opentracing.ErrSpanContextNotFound {
			grpclog.Errorf("extract from metadata err: %v", err)
		} else {
			span := tracer.StartSpan(
				info.FullMethod,
				ext.RPCServerOption(spanContext),
				opentracing.Tag{
					Key:   string(ext.Component),
					Value: "gRPC Server",
				},
				ext.SpanKindRPCServer,
			)
			defer span.Finish()
			ctx = opentracing.ContextWithSpan(ctx, span)
		}
		return handler(ctx, req)
	}
}
