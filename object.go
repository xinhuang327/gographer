package gographer

import (
	"errors"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"reflect"
	"runtime/debug"
)

func (sch *SchemaInfo) processObjectType(

	typ *TypeInfo,
	qlTypes map[string]*graphql.Object,
	qlConns map[string]*relay.GraphQLConnectionDefinitions,
	nodeDefinitions *relay.NodeDefinitions) *graphql.Object {

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
			returnType := funcType.Out(0) // only use first return value, TODO: handle error
			var fieldArgs graphql.FieldConfigArgument

			returnQLType, qlTypeKind := getComplexQLType(returnType, rf.Name, qlTypes, qlConns)

			resultIsConnection := qlTypeKind == QLTypeKind_Connection

			funcArgs := make(graphql.FieldConfigArgument)

			if rf.AutoArgs {
				// use struct args
				if funcType.NumIn() == 2 {
					argStructType := funcType.In(1)
					if argStructType.Kind() == reflect.Struct {
						for i := 0; i < argStructType.NumField(); i++ {

							argField := argStructType.Field(i)
							argFieldName := lowerFirst(argField.Name)
							argQLType := ToQLType(argField.Type)

							var defaultValue interface{} = nil
							if defTag := argField.Tag.Get(TAG_DefaultValue); defTag != "" {
								defaultValue = ParseString(defTag, argField.Type)
							}
							if nonNullTag := argField.Tag.Get(TAG_NonNull); nonNullTag == "true" {
								argQLType = graphql.NewNonNull(argQLType)
							}
							funcArgs[argFieldName] = &graphql.ArgumentConfig{
								Type:         argQLType,
								DefaultValue: defaultValue,
							}
						}
					} else {
						Warning("AutoArgs needs a struct value as argument", rf.MethodName)
					}
				}
			} else {
				// use manual argument info
				for i := 1; i < funcType.NumIn(); i++ {
					argQLType := ToQLType(funcType.In(i))
					arg := rf.Args[i-1]
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

func (sch *SchemaInfo) dynamicCallResolver(
	rf ResolvedFieldInfo,
	funcType reflect.Type,
	typ *TypeInfo,
	fieldArgs graphql.FieldConfigArgument,
	resultIsConnection bool,
	p graphql.ResolveParams) (interface{}, error) {

	defer func() {
		if e := recover(); e != nil {
			fmt.Printf("%s: %s", e, debug.Stack()) // line 20
		}
	}()

	fmt.Println("resultIsConnection", resultIsConnection)
	fmt.Println("[dynamicCallResolver]", "funcType=", funcType, "rf=", rf, "typ=", typ.Name)

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
		if funcType.NumIn() == 2{
			argStructType := funcType.In(1)
			argStructVal := reflect.New(argStructType).Elem()

			for i := 0; i < argStructVal.NumField(); i++ {
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
		}

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
