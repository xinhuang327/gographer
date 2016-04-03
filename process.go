package gographer

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"golang.org/x/net/context"
	"reflect"
	"fmt"
	"errors"
	"unicode/utf8"
	"unicode"
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

func (sch *SchemaInfo) processMutationType(typ *TypeInfo, qlTypes map[string]*graphql.Object, qlConns map[string]*relay.GraphQLConnectionDefinitions, nodeDefinitions *relay.NodeDefinitions) (*graphql.Object) {
	refType := typ.Type
	refPtrType := reflect.PtrTo(refType)

	var mutationFields = make(graphql.Fields)

	for _, mf := range typ.mutationFields {

		// try find method for pointer type first
		var method reflect.Method
		foundMethod := false
		if method, foundMethod = refPtrType.MethodByName(mf.MethodName); !foundMethod {
			method, foundMethod = refType.MethodByName(mf.MethodName)
		}

		if foundMethod {

			funcType := method.Func.Type()
			mutConf := relay.MutationConfig{}
			mutConf.Name = mf.MethodName

			var inputFields = make(graphql.InputObjectConfigFieldMap)
			for i := 1; i < funcType.NumIn(); i++ {
				argQLType := ToQLType(funcType.In(i)) // TODO: handle GraphQL ID type?
				arg := mf.Args[i - 1]
				if arg.NonNull {
					argQLType = graphql.NewNonNull(argQLType)
				}
				inputFields[arg.Name] = &graphql.InputObjectFieldConfig{
					Type:         argQLType,
					DefaultValue: arg.DefaultValue,
				}
			}
			mutConf.InputFields = inputFields

			mfCaptured := mf
			mutConf.MutateAndGetPayload = func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
				return sch.dynamicCallMutateAndGetPayload(mfCaptured, typ, inputMap)
			}

			var outputFields = make(graphql.Fields)

			for i := 0; i < funcType.NumOut(); i ++ {

				outInfo := mf.Outputs[i]
				outType := funcType.Out(i)
				isList := false

				if outType.Kind() == reflect.Ptr {
					outType = outType.Elem() // get output's underlying type if it's pointer type
				} else if outType.Kind() == reflect.Slice {
					isList = true
					outType = outType.Elem()
					// element type may be pointer
					if outType.Kind() == reflect.Ptr {
						outType = outType.Elem()
					}
				}

				var outQLType graphql.Output

				if outInfo.ElemInterface == nil {
					// use return type as output field type
					if outQLType = ToQLType(outType); outQLType == nil {
						var foundQLType bool
						if outQLType, foundQLType = qlTypes[outType.Name()]; !foundQLType {
							Warning("Cannot find QL type for return type ", outType, " in function: ", mf.MethodName)
						}
					}
					if isList {
						outQLType = graphql.NewList(outQLType)
					}
				} else {
					// so it's GraphQL's edge type or connection type
					// first, find or create connection for the element type
					elemType := reflect.TypeOf(outInfo.ElemInterface)
					var elemQLType graphql.Output
					var foundQLType bool
					if elemQLType, foundQLType = qlTypes[elemType.Name()]; !foundQLType {
						Warning("Cannot find QL type for element type ", outType, " in function: ", mf.MethodName)
					}

					conn := getOrCreateConnection(elemType.Name(), elemQLType, qlConns)
					if outType == reflect.TypeOf(relay.EdgeType{}) {
						outQLType = conn.EdgeType
					} else if outType == reflect.TypeOf(relay.Connection{}) {
						outQLType = conn.ConnectionType
					} else {
						Warning("Invalid output type ", outType.Name(), " for specified element type ", elemType.Name())
					}
				}

				outputFields[outInfo.Name] = &graphql.Field{
					Type: outQLType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						payload := p.Source.(map[string]interface{})
						output := payload[outInfo.Name]
						fmt.Println("Got output payload", outInfo.Name, output)
						return output, nil
					},
				}

			}
			mutConf.OutputFields = outputFields
			mutationFields[mf.Name] = relay.MutationWithClientMutationID(mutConf)
		} else {
			Warning("Cannot find method", mf.MethodName, "for type", refType.Name())
		}
	}

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: mutationFields,
	})
	return mutationType
}

func (sch *SchemaInfo) dynamicCallMutateAndGetPayload(mf MutationFieldInfo, typ *TypeInfo, inputMap map[string]interface{}) (map[string]interface{}, error) {
	mutVal := reflect.ValueOf(typ.instance)
	methodVal := mutVal.MethodByName(mf.MethodName)

	var inValues []reflect.Value
	for _, arg := range mf.Args {
		var argObj interface{}
		var hasInput bool
		if argObj, hasInput = inputMap[arg.Name]; !hasInput {
			argObj = arg.DefaultValue
		}
		inValues = append(inValues, reflect.ValueOf(argObj))
	}

	outValues := methodVal.Call(inValues) // call mutate function!

	outMap := make(map[string]interface{})
	for i, outInfo := range mf.Outputs {
		out := outValues[i].Interface()
		outMap[outInfo.Name] = out // set output fields map, will be sent to output fields resolver
	}
	return outMap, nil
}

func (sch *SchemaInfo) processObjectType(typ *TypeInfo, qlTypes map[string]*graphql.Object, qlConns map[string]*relay.GraphQLConnectionDefinitions, nodeDefinitions *relay.NodeDefinitions) (*graphql.Object) {
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
			funcType := method.Func.Type()

			// get QL type for return type
			returnType := funcType.Out(0) // ignore returned error, TODO: handle error
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

			var elemQLType graphql.Output

			elemTypeName := elemType.Name()
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
				resultIsConnection := false

				if !isList {
					returnQLType = elemQLType

				} else {
					if isPrimitive {
						// primitive list
						returnQLType = graphql.NewList(elemQLType)

					} else {
						// is connection
						resultIsConnection = true
						conn := getOrCreateConnection(elemTypeName, elemQLType, qlConns)
						returnQLType = conn.ConnectionType

						// bind the args for connection
						funcArgs := make(graphql.FieldConfigArgument)

						if rf.AutoArgs {

							// use struct args
							argStructType := funcType.In(1)
							if argStructType.Kind() == reflect.Struct {
								for i := 0; i < argStructType.NumField(); i ++ {

									argField := argStructType.Field(i)
									argFieldName := lowerFirst(argField.Name)
									argQLType := ToQLType(argField.Type)

									var defaultValue interface{} = nil
									if defTag := argField.Tag.Get("def"); defTag != "" {
										defaultValue = ParseString(defTag, argField.Type)
									}
									funcArgs[argFieldName] = &graphql.ArgumentConfig{
										Type:         argQLType,
										DefaultValue: defaultValue,
									}
								}
							} else {
								Warning("AutoArgs needs a struct value as argument", rf.MethodName)
							}
						} else {

							// use manual argument info
							for i := 1; i < funcType.NumIn(); i++ {
								argQLType := ToQLType(funcType.In(i))
								arg := rf.Args[i - 1]
								if arg.NonNull {
									argQLType = graphql.NewNonNull(argQLType)
								}
								funcArgs[arg.Name] = &graphql.ArgumentConfig{
									Type:         argQLType,
									DefaultValue: arg.DefaultValue,
								}
							}
						}
						fieldArgs = relay.NewConnectionArgs(funcArgs)
					}
				}
				// capture infomation for later function call
				typCaptured := typ
				rfCaptured := rf

				fields[rf.Name] = &graphql.Field{
					Type: returnQLType,
					Args: fieldArgs,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// call the function!
						return sch.dynamicCallResolver(rfCaptured, funcType, typCaptured, fieldArgs, resultIsConnection, p)
					},
				}

			} else {
				Warning("Cannot find QL Type for return type: ", returnType.Name(), "method:", rf.MethodName)
			}
		} else {
			Warning("Cannot find method", rf.MethodName, "for type", refType.Name())
		}
	} // end of resolved fields

	qlTypeConf.Fields = fields
	if !typ.isRootType {
		qlTypeConf.Interfaces = []*graphql.Interface{nodeDefinitions.NodeInterface}
	}
	qlType := graphql.NewObject(qlTypeConf)
	qlTypes[qlTypeConf.Name] = qlType

	return qlType
}

func (sch *SchemaInfo) dynamicCallResolver(rf ResolvedFieldInfo, funcType reflect.Type, typ *TypeInfo, fieldArgs graphql.FieldConfigArgument, resultIsConnection bool, p graphql.ResolveParams) (interface{}, error) {
	fmt.Println("resultIsConnection", resultIsConnection)
	fmt.Println("[dynamicCallResolver]", "funcType=", funcType, "rf=", rf, "typ=", typ, "p=", p)

	var objVal reflect.Value
	if typ.isRootType {
		objVal = reflect.ValueOf(sch.rootInstance)
	} else {
		objVal = reflect.ValueOf(p.Source) // p.Source would be a pointer to struct
	}
	if !objVal.IsValid() {
		return nil, errors.New("Cannot get source object when calling " + rf.MethodName)
	}

	methodVal := objVal.MethodByName(rf.MethodName)
	if !methodVal.IsValid() {
		return nil, errors.New(fmt.Sprint("Cannot get method ", rf.MethodName, " for object ", objVal.Type()))
	}

	var inValues []reflect.Value
	if rf.AutoArgs {
		// use struct args
		argStructType := funcType.In(1)
		argStructVal := reflect.New(argStructType).Elem()

		for i := 0; i < argStructVal.NumField(); i ++ {
			argStructField := argStructType.Field(i)
			argStructFieldVal := argStructVal.Field(i)
			lowerFirstFieldName := lowerFirst(argStructField.Name)

			var argObj interface{} = nil
			var hasInput bool
			if argObj, hasInput = p.Args[lowerFirstFieldName]; !hasInput {
				argObj = fieldArgs[lowerFirstFieldName].DefaultValue
			}
			if argObj != nil {
				argStructFieldVal.Set(reflect.ValueOf(argObj)) // bind field value
			}
		}
		inValues = append(inValues, argStructVal)

	} else {
		// use plain args
		for _, arg := range rf.Args {
			var argObj interface{}
			var hasInput bool
			if argObj, hasInput = p.Args[arg.Name]; !hasInput {
				argObj = arg.DefaultValue
			}
			inValues = append(inValues, reflect.ValueOf(argObj))
		}
	}

	outValues := methodVal.Call(inValues)

	out := outValues[0].Interface()

	fmt.Println(funcType, rf.MethodName, "Out:", out)

	if resultIsConnection {
		resultSlice := toEmptyInterfaceSlice(out)
		// TODO: manage pagination
		return relay.ConnectionFromArray(resultSlice, relay.NewConnectionArguments(p.Args)), nil
	} else {
		return out, nil
	}
}

func getOrCreateConnection(elemTypeName string, elemQLType graphql.Output, qlConns map[string]*relay.GraphQLConnectionDefinitions) *relay.GraphQLConnectionDefinitions {
	var conn *relay.GraphQLConnectionDefinitions
	var found bool

	if conn, found = qlConns[elemTypeName]; !found {
		conn = relay.ConnectionDefinitions(relay.ConnectionConfig{
			Name:     elemTypeName,
			NodeType: elemQLType.(*graphql.Object),
		})
		qlConns[elemTypeName] = conn
	}

	return conn
}

func toEmptyInterfaceSlice(slice interface{}) []interface{} {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}

	ret := make([]interface{}, s.Len())
	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}
	return ret
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}