package gographer

import (
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"golang.org/x/net/context"
	"reflect"
	"strings"
)

func (sch *SchemaInfo) processMutationType(
	typ *TypeInfo,
	qlTypes map[string]*graphql.Object,
	qlConns map[string]*relay.GraphQLConnectionDefinitions,
	nodeDefinitions *relay.NodeDefinitions) *graphql.Object {

	if len(typ.mutationFields) == 0 {
		return nil
	}

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

			if mf.AutoArgs {
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
							inputFields[argFieldName] = &graphql.InputObjectFieldConfig{
								Type:         argQLType,
								DefaultValue: defaultValue,
							}
						}
					} else {
						Warning("AutoArgs needs a struct value as argument", mf.MethodName, argStructType, argStructType.Kind())
					}
				}

			} else {
				for i := 1; i < funcType.NumIn(); i++ {
					argQLType := ToQLType(funcType.In(i)) // TODO: handle GraphQL ID type?
					arg := mf.Args[i-1]
					if arg.NonNull {
						argQLType = graphql.NewNonNull(argQLType)
					}
					inputFields[arg.Name] = &graphql.InputObjectFieldConfig{
						Type:         argQLType,
						DefaultValue: arg.DefaultValue,
					}
				}
			}
			mutConf.InputFields = inputFields

			var outputFields = make(graphql.Fields)

			//var outTypes []reflect.Type
			var outQLTypes []graphql.Output
			var outputInfos []OutputInfo

			if mf.AutoOutputs {

				// use struct args to infer output field types
				outStructType := funcType.Out(0)
				if outStructType.Kind() == reflect.Ptr {
					outStructType = outStructType.Elem() // return type may be pointer type to struct
				}
				if outStructType.Kind() == reflect.Struct {

					for i := 0; i < outStructType.NumField(); i++ {

						outField := outStructType.Field(i)
						outFieldName := lowerFirst(outField.Name)
						outQLType, qlTypeKind := getComplexQLType(outField.Type, outField.Name, qlTypes, qlConns) // use full name to infer type

						var qlFieldName string
						if jsonTag := outField.Tag.Get("json"); jsonTag != "" {
							qlFieldName = jsonTag
						} else {
							qlFieldName = outFieldName
						}

						outInfo := OutputInfo{
							Name: qlFieldName,
						}

						if qlTypeKind == QLTypeKind_Edge {
							if strings.HasSuffix(outField.Name, "Edge") {
								outInfo.ElemTypeName = strings.TrimSuffix(outField.Name, "Edge")
							} else {
								Warning("Invalid field name for EdgeType, need to specify struct type", outField.Name)
							}
						} else if qlTypeKind == QLTypeKind_Connection {
							if strings.HasSuffix(outField.Name, "Connection") {
								outInfo.ElemTypeName = strings.TrimSuffix(outField.Name, "Connection")
							} else {
								Warning("Invalid field name for ConnectionType, need to specify struct type", outField.Name)
							}
						}

						outQLTypes = append(outQLTypes, outQLType)
						outputInfos = append(outputInfos, outInfo)
					}
					mf.Outputs = outputInfos // save information for dynamicCallMutateAndGetPayload
				} else {
					Warning("AutoOutputs needs a struct value as argument", mf.MethodName, outStructType, outStructType.Kind())
				}

			} else {
				// use manually OutputInfo and function type's output information
				for i := 0; i < funcType.NumOut(); i++ {
					outputInfo := mf.Outputs[i]
					outQLType, _ := getComplexQLType(funcType.Out(i), outputInfo.Name, qlTypes, qlConns)
					outQLTypes = append(outQLTypes, outQLType)
					outputInfos = append(outputInfos, outputInfo)
				}
			}

			for i := 0; i < len(outputInfos); i++ {

				outInfo := outputInfos[i]
				outQLType := outQLTypes[i]

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

			mfCaptured := mf
			mutConf.MutateAndGetPayload = func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
				return sch.dynamicCallMutateAndGetPayload(mfCaptured, funcType, typ, inputFields, inputMap)
			}

			mutationFields[mf.Name] = relay.MutationWithClientMutationID(mutConf)
		} else {
			Warning("Cannot find method", mf.MethodName, "for type", refType.Name())
		}
	}

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: mutationFields,
	})
	return mutationType
}

func (sch *SchemaInfo) dynamicCallMutateAndGetPayload(
	mf MutationFieldInfo,
	funcType reflect.Type,
	typ *TypeInfo,
	inputFields graphql.InputObjectConfigFieldMap,
	inputMap map[string]interface{}) (map[string]interface{}, error) {

	fmt.Println("[dynamicCallMutateAndGetPayload]", "funcType=", funcType, "mf=", mf, "typ=", typ.Name)

	mutVal := reflect.ValueOf(typ.instance)
	methodVal := mutVal.MethodByName(mf.MethodName)

	var inValues []reflect.Value

	if mf.AutoArgs {
		// use struct args
		if funcType.NumIn() == 2 {
			argStructType := funcType.In(1)
			argStructVal := reflect.New(argStructType).Elem()

			for i := 0; i < argStructVal.NumField(); i++ {
				argStructField := argStructType.Field(i)
				argStructFieldVal := argStructVal.Field(i)
				lowerFirstFieldName := lowerFirst(argStructField.Name)

				var argObj interface{} = nil
				var hasInput bool
				if argObj, hasInput = inputMap[lowerFirstFieldName]; !hasInput {
					argObj = inputFields[lowerFirstFieldName].DefaultValue
				}
				if argObj != nil {
					argStructFieldVal.Set(reflect.ValueOf(argObj)) // bind field value
				}
			}
			inValues = append(inValues, argStructVal)
		}

	} else {
		// use plain args
		for _, arg := range mf.Args {
			var argObj interface{}
			var hasInput bool
			if argObj, hasInput = inputMap[arg.Name]; !hasInput {
				argObj = arg.DefaultValue
			}
			inValues = append(inValues, reflect.ValueOf(argObj))
		}
	}

	outValues := methodVal.Call(inValues) // call mutate function!

	outMap := make(map[string]interface{})

		for i, outInfo := range mf.Outputs {
			// set output fields map, will be sent to output fields resolver
			if mf.AutoOutputs {
				outMap[outInfo.Name] = outValues[0].Elem().Field(i).Interface() // extract field value from output struct
			} else {
				outMap[outInfo.Name] = outValues[i].Interface()
			}
		}

	return outMap, nil
}
