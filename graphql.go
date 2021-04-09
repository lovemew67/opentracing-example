package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/graph-gophers/graphql-go/trace"
	mq "github.com/machinebox/graphql"
	"github.com/opentracing/opentracing-go"
)

type query struct{}

func (_ *query) Hello() string { return "Hello, world!" }

func startUpGraphQL() {
	go func() {
		time.Sleep(5 * time.Second)

		// define a Context for the request
		ctx := context.Background()

		// create a client (safe to share across requests)
		client := mq.NewClient("http://localhost:8080/query")

		// make a request 1
		sp1 := opentracing.StartSpan(fmt.Sprintf("graphql client 1: %d", time.Now().UnixNano()))
		defer sp1.Finish()
		req1 := mq.NewRequest(`
			query {
				hello
			}
		`)
		req1.Var("key", "value")
		sp1.SetTag("key", "value")
		req1.Header.Set("Cache-Control", "no-cache")
		err := sp1.Tracer().Inject(sp1.Context(), opentracing.TextMap, opentracing.HTTPHeadersCarrier(req1.Header))
		if err != nil {
			log.Fatalf("couldn't inject headers (%+v)", err)
		}

		// make a request 2
		sp2 := opentracing.StartSpan(fmt.Sprintf("graphql client 2: %d", time.Now().UnixNano()))
		defer sp2.Finish()
		req2 := mq.NewRequest(`
			query {
				hello
			}
		`)
		req2.Var("key", "value")
		sp2.SetTag("key", "value")
		req2.Header.Set("Cache-Control", "no-cache")
		err = sp2.Tracer().Inject(sp2.Context(), opentracing.TextMap, opentracing.HTTPHeadersCarrier(req2.Header))
		if err != nil {
			log.Fatalf("couldn't inject headers (%+v)", err)
		}

		// run it and capture the response
		var respData map[string]interface{}

		if err := client.Run(ctx, req1, &respData); err != nil {
			log.Printf("failed run client, err: %+v\n", err)
		} else {
			log.Printf("resp: %+v\n", respData)
		}

		if err := client.Run(ctx, req2, &respData); err != nil {
			log.Printf("failed run client, err: %+v\n", err)
		} else {
			log.Printf("resp: %+v\n", respData)
		}
	}()

	s := `
		type Query {
			hello: String!
		}
	`
	schema := graphql.MustParseSchema(s, &query{}, graphql.Tracer(trace.OpenTracingTracer{}))
	http.Handle("/query", &relay.Handler{Schema: schema})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
