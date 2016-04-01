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
	qlConns := make(map[string]*relay.GraphQLConnectionDefinitions)
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
				isList := returnType.Kind() == reflect.Slice
				isPtr := returnType.Kind() == reflect.Ptr
				elemType := returnType
				if isList || isPtr {
					elemType = returnType.Elem()
					// in case of slice of struct pointers
					if elemType.Kind() == reflect.Ptr {
						elemType = elemType.Elem()
					}
				}
				elemTypeName := elemType.Name()
				var elemQLType graphql.Output
				isPrimitive := true
				if elemQLType = ToQLType(elemType); elemQLType == nil {
					isPrimitive = false
					if qlType, ok := qlTypes[elemTypeName]; ok {
						elemQLType = qlType
					}
				}
				if elemQLType != nil {
					var fieldArgs graphql.FieldConfigArgument
					var returnQLType graphql.Output
					if !isList {
						returnQLType = elemQLType
					} else {
						if isPrimitive {
							// primitive list
							returnQLType = graphql.NewList(elemQLType)
						} else {
							// find or create connection type
							var conn *relay.GraphQLConnectionDefinitions
							var found bool
							if conn, found = qlConns[elemTypeName]; !found {
								conn = relay.ConnectionDefinitions(relay.ConnectionConfig{
									Name:     elemTypeName,
									NodeType: elemQLType.(*graphql.Object),
								})
								qlConns[elemTypeName] = conn
							}
							returnQLType = conn.ConnectionType
							funcArgs := make(graphql.FieldConfigArgument)
							for i := 1; i < funcType.NumIn(); i++ {
								argQLType := ToQLType(funcType.In(i))
								funcArgs[rf.ArgNames[i - 1]] = &graphql.ArgumentConfig{
									Type:         argQLType,
									DefaultValue: nil,
								}
							}
							fieldArgs = relay.NewConnectionArgs(funcArgs)
						}
					}
					fields[rf.Name] = &graphql.Field{
						Type: returnQLType,
						Args: fieldArgs,
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return dynamicCallResolver(funcType, p)
						},
					}
				} else {
					Warning("Cannot find QL Type for return type: ", returnType.Name(), "method:", rf.MethodName)
				}
			} else {
				Warning("Cannot find method", rf.MethodName, "for type", refType.Name())
			}
		}
		qlTypeConf.Fields = fields
		if !typ.isRootType {
			qlTypeConf.Interfaces = []*graphql.Interface{nodeDefinitions.NodeInterface}
		}
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