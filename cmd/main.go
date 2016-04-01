package main

import (
	"github.com/xinhuang327/gographer/cmd/data"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"log"
	"io/ioutil"
	"os"
	"encoding/json"
)

func main() {

	schemaInfo := data.GetModelSchemaInfo()
	schema, err := schemaInfo.GetSchema()
	if err != nil {
		fmt.Println("Error", err)
	} else {
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: testutil.IntrospectionQuery,
		})
		if result.HasErrors() {
			log.Fatalf("ERROR introspecting schema: %v", result.Errors)
			return
		} else {
			b, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				log.Fatalf("ERROR: %v", err)
			}
			err = ioutil.WriteFile("schema.json", b, os.ModePerm)
			if err != nil {
				log.Fatalf("ERROR: %v", err)
			}
		}
	}

}
