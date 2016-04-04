package gographer

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"golang.org/x/net/context"
	"reflect"
)

func (sch SchemaInfo) GetSchema() (graphql.Schema, error) {

	qlTypes := make(map[string]*graphql.Object)
	qlConns := make(map[string]*relay.GraphQLConnectionDefinitions)
	var rootType *graphql.Object
	var mutationType *graphql.Object

	var nodeDefinitions *relay.NodeDefinitions
	var schema graphql.Schema

	// register all the types
	nodeDefinitions = relay.NewNodeDefinitions(relay.NodeDefinitionsConfig{

		IDFetcher: func(id string, info graphql.ResolveInfo, ctx context.Context) (interface{}, error) {
			resolvedID := relay.FromGlobalID(id)
			if typ, ok := sch.typesByName[resolvedID.Type]; ok {
				return typ.idResolver(resolvedID.ID), nil
			}
			return nil, nil
		},

		TypeResolve: func(value interface{}, info graphql.ResolveInfo) *graphql.Object {
			type_ := reflect.ValueOf(value).Elem().Type()
			if qlType, ok := qlTypes[type_.Name()]; ok {
				return qlType
			} else {
				Warning("[GetSchema Error]", "cannot resolve type", value)
				return nil
			}
		},
	})

	// process all the object types, object types must be registered in order of dependency at the time
	for _, typ := range sch.types {
		if !typ.isMutationType {

			qlType := sch.processObjectType(typ, qlTypes, qlConns, nodeDefinitions)
			if typ.isRootType {
				rootType = qlType
				sch.rootInstance = typ.instance
			}
		}
	}

	// process mutation type, should have only one mutation type
	for _, typ := range sch.types {
		if typ.isMutationType {

			mutType := sch.processMutationType(typ, qlTypes, qlConns, nodeDefinitions)
			mutationType = mutType
			sch.mutationInstance = typ.instance
		}
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootType,
		Mutation: mutationType,
	})
	return schema, err
}
