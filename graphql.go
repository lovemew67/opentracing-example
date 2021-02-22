package main

import (
	"context"
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

		sp := opentracing.StartSpan("graphql client") // Start a new root span.
		defer sp.Finish()

		// create a client (safe to share across requests)
		client := mq.NewClient("http://localhost:8080/query")

		// make a request
		req := mq.NewRequest(`
			query {
				hello
			}
		`)

		// set any variables
		req.Var("key", "value")
		sp.SetTag("key", "value")

		// set header fields
		req.Header.Set("Cache-Control", "no-cache")

		// inject span
		err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.HTTPHeadersCarrier(req.Header))
		if err != nil {
			log.Fatalf("couldn't inject headers (%+v)", err)
		}

		// define a Context for the request
		ctx := context.Background()

		// run it and capture the response
		var respData map[string]interface{}
		if err := client.Run(ctx, req, &respData); err != nil {
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
