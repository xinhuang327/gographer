package gographer

import (
	"encoding"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"reflect"
	"strings"
)

const (
	TAG_DefaultValue = "def"
	TAG_NonNull      = "nonNull"
)

const (
	QLTypeKind_Simple     = "QLTYPE_Simple"
	QLTypeKind_SimpleList = "QLTYPE_SimpleList"
	QLTypeKind_Struct     = "QLTypeKind_Struct"
	QLTypeKind_Connection = "QLTYPE_Connection"
	QLTypeKind_Edge       = "QLTYPE_Edge"
)

type QLTypeKind string

type SchemaInfo struct {
	types            []*TypeInfo
	typesByName      map[string]*TypeInfo
	rootInstance     interface{}
	mutationInstance interface{}
}

func NewSchemaInfo() *SchemaInfo {
	return &SchemaInfo{
		typesByName: make(map[string]*TypeInfo),
	}
}

func (sch *SchemaInfo) RegType(instance interface{}) *TypeInfo {
	typeDef := NewTypeInfo(instance)
	sch.types = append(sch.types, typeDef)
	sch.typesByName[typeDef.Name] = typeDef
	return typeDef
}

type TypeInfo struct {
	Name           string
	Type           reflect.Type
	idResolver     IDResolver
	fields         graphql.Fields
	resolvedFields []ResolvedFieldInfo
	mutationFields []MutationFieldInfo
	isRootType     bool
	isMutationType bool
	instance       interface{}
	isNonNode      bool
	embeddedTypes  map[string]reflect.Type
}

type IDResolver func(id string) interface{}

func NewTypeInfo(instance interface{}) *TypeInfo {
	type_ := reflect.TypeOf(instance)
	if type_.Kind() == reflect.Ptr {
		type_ = type_.Elem()
	}
	typeDef := TypeInfo{
		Type:          type_,
		Name:          type_.Name(),
		fields:        make(graphql.Fields),
		instance:      instance,
		embeddedTypes: make(map[string]reflect.Type),
	}
	return &typeDef
}

func (typ *TypeInfo) SetNonNode() *TypeInfo {
	typ.isNonNode = true
	return typ
}

func (typ *TypeInfo) SetIDResolver(f IDResolver) *TypeInfo {
	typ.idResolver = f
	return typ
}

func (typ *TypeInfo) SetEmbeddedTypes(ifaces ...interface{}) *TypeInfo {
	for _, iface := range ifaces {
		t := reflect.TypeOf(iface)
		typ.embeddedTypes[t.Name()] = t
	}
	return typ
}

func (typ *TypeInfo) SetRoot() *TypeInfo {
	typ.isRootType = true
	return typ
}

func (typ *TypeInfo) SimpleField(name string) *TypeInfo {
	for i := 0; i < typ.Type.NumField(); i++ {
		field := typ.Type.Field(i)
		if field.Name == name || field.Tag.Get("json") == name {
			if qlType := ToQLType(field.Type); qlType != nil {
				return typ.AddField(name, &graphql.Field{
					Type: qlType,
				})
			}
		}
	}
	Warning("SimpleField not found", typ.Name, name)
	return typ
}

func (typ *TypeInfo) IDField(name string, idFetcher relay.GlobalIDFetcherFn) *TypeInfo {
	for i := 0; i < typ.Type.NumField(); i++ {
		field := typ.Type.Field(i)
		if field.Name == name || field.Tag.Get("json") == name {
			return typ.AddField(name, relay.GlobalIDField(typ.Name, idFetcher))
		}
	}
	Warning("IDField not found", typ.Name, name)
	return typ
}

var TextMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

// Auto adds simple fields, including embedded struct's fields (implemented with resolved field)
func (typ *TypeInfo) SimpleFields() *TypeInfo {

	typ.processSimpleFields([]string{}, typ.Type)

	return typ
}

// Recursively process all the fields, including embedded struct fields.
func (typ *TypeInfo) processSimpleFields(nestFieldsInput []string, nestTypeInput reflect.Type) {
	//fmt.Println("processSimpleFields", nestTypeInput, nestFieldsInput)
	var nestFields []string
	for _, nf := range nestFieldsInput {
		nestFields = append(nestFields, nf)
	}
	var nestType = nestTypeInput

	for i := 0; i < nestType.NumField(); i++ {
		field := nestType.Field(i)
		fmt.Println(nestType.Name(), i, field.Name, field.Type)
		var fullFieldName = field.Name
		var fieldName string
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			fieldName = jsonTag
		} else {
			fieldName = field.Name
		}

		hasQLType := false
		var qlType graphql.Output
		if qlType = ToQLType(field.Type); qlType != nil {
			if len(nestFields) == 0 {
				typ.AddField(fieldName, &graphql.Field{
					Type: qlType,
				})
			} else {
				typ.resolvedFields = append(typ.resolvedFields, ResolvedFieldInfo{
					Name:       fieldName,
					Args:       nil,
					AutoArgs:   true,
					ManualType: qlType,
					ExtensionFunc: func(s interface{}) interface{} {
						val := reflect.ValueOf(s)
						// iterate field value chain
						for _, nf := range nestFields {
							val = val.FieldByName(nf)
						}
						fieldValue := val.FieldByName(fullFieldName)
						if fieldValue.IsValid() {
							return fieldValue.Interface()
						}
						return nil
					},
				})
			}
			hasQLType = true
		}

		if !hasQLType {
			// deal with struct field...
			if field.Type.Kind() == reflect.Struct {
				fmt.Println("Struct Field: ", typ.Name, field.Name, field.Type.Name())

				if field.Type.Implements(TextMarshalerType) {
					// time.Time
					//fmt.Println("is TextMarshalerType")
					fullFieldName := field.Name
					typ.ExtensionField(fieldName, func(s interface{}) string {
						val := reflect.ValueOf(s)
						// iterate field value chain
						for _, nf := range nestFields {
							val = val.FieldByName(nf)
						}
						fieldValue := val.FieldByName(fullFieldName)
						if fieldValue.IsValid() {
							if textMarshaler, ok := fieldValue.Interface().(encoding.TextMarshaler); ok {
								text, _ := textMarshaler.MarshalText()
								return string(text)
							}
						}
						return ""
					}, AutoArgs)

				} else {
					// handle embedded struct
					if field.Name == field.Type.Name() && field.Type == typ.embeddedTypes[field.Name] {
						var nextNestFields []string
						for _, nf := range nestFields {
							nextNestFields = append(nextNestFields, nf)
						}
						nextNestFields = append(nextNestFields, field.Name)
						//nestType = field.Type will set previous call's nestType, don't do it.
						typ.processSimpleFields(nextNestFields, field.Type)
					}
				}
			}
		}
	}
}

func (typ *TypeInfo) ResolvedField(name string, methodName string, args []ArgInfo) *TypeInfo {
	autoArgs := IsAutoArgs(args)
	if autoArgs {
		args = nil
	}
	typ.resolvedFields = append(typ.resolvedFields, ResolvedFieldInfo{
		Name:       name,
		MethodName: methodName,
		Args:       args,
		AutoArgs:   autoArgs,
	})
	return typ
}

func (typ *TypeInfo) ExtensionField(name string, extensionFunc interface{}, args []ArgInfo) *TypeInfo {
	autoArgs := IsAutoArgs(args)
	if autoArgs {
		args = nil
	}
	typ.resolvedFields = append(typ.resolvedFields, ResolvedFieldInfo{
		Name:          name,
		ExtensionFunc: extensionFunc,
		Args:          args,
		AutoArgs:      autoArgs,
	})
	return typ
}

// Auto adds resolved fields
func (typ *TypeInfo) ResolvedFields() *TypeInfo {
	ptrType := reflect.PtrTo(typ.Type)
	for i := 0; i < ptrType.NumMethod(); i++ {
		method := ptrType.Method(i)
		var methodName = method.Name
		if strings.HasPrefix(methodName, "Get") {
			fieldName := lowerFirst(strings.TrimPrefix(methodName, "Get"))
			typ.ResolvedField(fieldName, methodName, AutoArgs)
		}
	}
	return typ
}

type ArgInfo struct {
	Name         string
	DefaultValue interface{}
	NonNull      bool
}

var AutoArgs = []ArgInfo{ArgInfo{"__AutoArgs__", nil, false}}

func IsAutoArgs(args []ArgInfo) bool {
	return len(args) == 1 && args[0] == AutoArgs[0]
}

func (typ *TypeInfo) AddField(name string, field *graphql.Field) *TypeInfo {
	typ.fields[name] = field
	return typ
}

type ResolvedFieldInfo struct {
	Name          string
	MethodName    string
	Args          []ArgInfo
	AutoArgs      bool
	ExtensionFunc interface{}
	ManualType    graphql.Output
}

func (typ *TypeInfo) SetMutation() *TypeInfo {
	typ.isMutationType = true
	return typ
}

func (typ *TypeInfo) MutationField(name string, methodName string, args []ArgInfo, outputs []OutputInfo) *TypeInfo {
	autoArgs := IsAutoArgs(args)
	if autoArgs {
		args = nil
	}
	autoOutputs := IsAutoOutputs(outputs)
	if autoOutputs {
		outputs = nil
	}
	typ.mutationFields = append(typ.mutationFields, MutationFieldInfo{
		Name:        name,
		MethodName:  methodName,
		Args:        args,
		AutoArgs:    autoArgs,
		Outputs:     outputs,
		AutoOutputs: autoOutputs,
	})
	return typ
}

// Auto adds mutation fields
func (typ *TypeInfo) MutationFields() *TypeInfo {
	ptrType := reflect.PtrTo(typ.Type)
	for i := 0; i < ptrType.NumMethod(); i++ {
		method := ptrType.Method(i)
		var methodName = method.Name
		var qlMutationName = lowerFirst(methodName)
		typ.MutationField(qlMutationName, methodName, AutoArgs, AutoOutputs)
	}
	return typ
}

type MutationFieldInfo struct {
	Name        string
	MethodName  string
	Args        []ArgInfo
	AutoArgs    bool
	Outputs     []OutputInfo
	AutoOutputs bool
}

type OutputInfo struct {
	Name          string
	ElemInterface interface{}
	ElemTypeName  string
}

func (outputInfo OutputInfo) GetElementTypeName() string {
	if outputInfo.ElemTypeName != "" {
		return outputInfo.ElemTypeName
	} else if outputInfo.ElemInterface != nil {
		return reflect.TypeOf(outputInfo.ElemInterface).Name()
	} else {
		return ""
	}
}

var AutoOutputs = []OutputInfo{OutputInfo{Name: "__AutoOutputs__"}}

func IsAutoOutputs(outputs []OutputInfo) bool {
	return len(outputs) == 1 && outputs[0] == AutoOutputs[0]
}
