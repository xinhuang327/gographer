package gographer

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"golang.org/x/net/context"
	"reflect"
	"fmt"
)

func (sch SchemaInfo) GetSchema() (graphql.Schema, error) {

	qlTypes := make(map[string]*graphql.Object)
	//qlConns := make(map[string]*relay.GraphQLConnectionDefinitions)
	var rootType *graphql.Object

	var nodeDefinitions *relay.NodeDefinitions
	var schema graphql.Schema

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

	for _, typ := range sch.types {
		qlTypeConf := graphql.ObjectConfig{}
		qlTypeConf.Name = typ.Name
		fields := make(graphql.Fields)
		// simple fields
		for fieldName, field := range typ.fields {
			fields[fieldName] = field
		}
		// node field for root
		if typ.isRootType {
			fields["node"] = nodeDefinitions.NodeField
		}
		// resolved fields
		for _, rf := range typ.resolvedFields {
			refType := typ.Type
			refPtrType := reflect.PtrTo(refType)
			// try find method for pointer type first
			var method reflect.Method
			foundMethod := false
			if method, foundMethod = refPtrType.MethodByName(rf.MethodName); !foundMethod {
				method, foundMethod = refType.MethodByName(rf.MethodName)
			}
			if foundMethod {
				// get QL type for return type
				funcType := method.Func.Type()
				returnType := funcType.Out(0) // ignore returned error
				isList := returnType.Kind() == reflect.Array
				isPtr := returnType.Kind() == reflect.Ptr
				if isList {

				} else if isPtr {
					returnType = returnType.Elem()
				}
				var returnQLType graphql.Output
				if returnQLType = ToQLType(returnType); returnQLType == nil {
					if qlType, ok := qlTypes[returnType.Name()]; ok {
						returnQLType = qlType
					}
				}
				if returnQLType != nil {
					fmt.Println("returnQLType", returnQLType)
					fields[rf.Name] = &graphql.Field{
						Type: returnQLType,
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return dynamicCallResolver(funcType, p)
						},
					}
				} else {
					Warning("Cannot find QL Type for", returnType.Name(), rf.MethodName)
				}
			} else {
				Warning("Cannot find method", rf.MethodName, "for type", refType.Name())
			}
		}
		qlTypeConf.Fields = fields
		qlType := graphql.NewObject(qlTypeConf)
		qlTypes[qlTypeConf.Name] = qlType
		if typ.isRootType {
			rootType = qlType
		}
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootType,
		Mutation: nil,
	})
	return schema, err
}

func dynamicCallResolver(funcType reflect.Type, p graphql.ResolveParams) (interface{}, error) {
	fmt.Println("[dynamicCallResolver]", funcType, p)
	return nil, nil
}