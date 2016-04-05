package main

import (
	"encoding/json"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/xinhuang327/gographer/cmd/data"
	"io/ioutil"
	"log"
	"os"
	"reflect"
)

func main() {
	inspectFunc(func(a int, b string) string {
		return "hello"
	})
}

func inspectFunc(fun interface{}) {
	typ := reflect.TypeOf(fun)
	fmt.Println(typ)
}

func updateSchema(){
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