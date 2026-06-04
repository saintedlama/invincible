package api

import "github.com/getkin/kin-openapi/openapi3"

func buildSpec() *openapi3.T {
	nameParam := openapi3.NewPathParameter("name").
		WithDescription("Process name").
		WithSchema(openapi3.NewStringSchema())

	processStatusSchema := openapi3.NewObjectSchema().
		WithProperty("Name", openapi3.NewStringSchema()).
		WithProperty("State", openapi3.NewStringSchema()).
		WithProperty("PID", openapi3.NewIntegerSchema())

	okSchema := openapi3.NewObjectSchema().WithProperty("ok", openapi3.NewStringSchema())

	jsonContent := func(schema *openapi3.Schema) openapi3.Content {
		return openapi3.NewContentWithJSONSchema(schema)
	}

	response200 := func(schema *openapi3.Schema) *openapi3.ResponseRef {
		return &openapi3.ResponseRef{Value: openapi3.NewResponse().
			WithDescription("OK").
			WithContent(jsonContent(schema))}
	}

	op := func(summary string, params openapi3.Parameters, resp *openapi3.ResponseRef) *openapi3.Operation {
		o := openapi3.NewOperation()
		o.Summary = summary
		o.Parameters = params
		o.Responses = openapi3.NewResponses()
		o.Responses.Set("200", resp)
		return o
	}

	nameParams := openapi3.Parameters{
		{Value: nameParam},
	}

	logEntrySchema := openapi3.NewObjectSchema().
		WithProperty("time", openapi3.NewStringSchema()).
		WithProperty("line", openapi3.NewStringSchema()).
		WithProperty("source", openapi3.NewStringSchema())

	logsSchema := openapi3.NewArraySchema().WithItems(logEntrySchema)
	nQueryParam := openapi3.NewQueryParameter("n").
		WithDescription("Number of log lines to return (default 100)").
		WithSchema(openapi3.NewIntegerSchema())
	formatQueryParam := openapi3.NewQueryParameter("format").
		WithDescription("Response format: omit for JSON array, use 'text' for newline-separated plain text (convenient for curl and agents).").
		WithSchema(openapi3.NewStringSchema())

	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Invincible API",
			Description: "Process manager HTTP API",
			Version:     "0.1.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath("/processes", &openapi3.PathItem{
				Get: op("List all processes", nil, response200(openapi3.NewArraySchema().WithItems(processStatusSchema))),
			}),
			openapi3.WithPath("/processes/{name}", &openapi3.PathItem{
				Get: op("Get a process by name", nameParams, response200(processStatusSchema)),
			}),
			openapi3.WithPath("/processes/{name}/logs", &openapi3.PathItem{
				Get: func() *openapi3.Operation {
					o := op("Get recent log lines for a process",
						openapi3.Parameters{{Value: nameParam}, {Value: nQueryParam}, {Value: formatQueryParam}},
						response200(logsSchema))
					return o
				}(),
			}),
			openapi3.WithPath("/processes/{name}/start", &openapi3.PathItem{
				Post: op("Start a process", nameParams, response200(okSchema)),
			}),
			openapi3.WithPath("/processes/{name}/stop", &openapi3.PathItem{
				Post: op("Stop a process", nameParams, response200(okSchema)),
			}),
			openapi3.WithPath("/processes/{name}/restart", &openapi3.PathItem{
				Post: op("Restart a process", nameParams, response200(okSchema)),
			}),
		),
	}
	return spec
}
