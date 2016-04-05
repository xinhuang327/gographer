package gographer

import (
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"reflect"
	"strconv"
	"strings"
	"runtime"
	"runtime/debug"
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
}

type IDResolver func(id string) interface{}

func NewTypeInfo(instance interface{}) *TypeInfo {
	type_ := reflect.TypeOf(instance)
	if type_.Kind() == reflect.Ptr {
		type_ = type_.Elem()
	}
	typeDef := TypeInfo{
		Type:     type_,
		Name:     type_.Name(),
		fields:   make(graphql.Fields),
		instance: instance,
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

// Auto adds simple fields
func (typ *TypeInfo) SimpleFields() *TypeInfo {
	for i := 0; i < typ.Type.NumField(); i++ {
		field := typ.Type.Field(i)
		var fieldName string
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			fieldName = jsonTag
		} else {
			fieldName = field.Name
		}
		if _, exists := typ.fields[fieldName]; !exists {
			if qlType := ToQLType(field.Type); qlType != nil {
				typ.AddField(fieldName, &graphql.Field{
					Type: qlType,
				})
			}
		}
	}
	return typ
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
		Name:       name,
		ExtensionFunc: extensionFunc,
		Args:       args,
		AutoArgs:   autoArgs,
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

func ToQLType(typ reflect.Type) graphql.Output {
	switch typ.Kind() {
	case reflect.Slice: // []string
		elemType := typ.Elem()
		if elemQLType := ToQLType(elemType); elemQLType != nil {
			return graphql.NewList(elemQLType)
		} else {
			return nil
		}
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		return graphql.Float
	case reflect.String:
		return graphql.String
	case reflect.Bool:
		return graphql.Boolean
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		return graphql.Int
	default:
		return nil
	}
}

func ParseString(str string, typ reflect.Type) interface{} {
	switch typ.Kind() {
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		if v, err := strconv.ParseFloat(str, 32); err == nil {
			return v
		}
	case reflect.String:
		return str
	case reflect.Bool:
		if v, err := strconv.ParseBool(str); err == nil {
			return v
		}
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		if v, err := strconv.ParseInt(str, 0, 0); err == nil {
			return v
		}
	default:
		return nil
	}
	return nil
}

func Warning(a ...interface{}) {
	_, file, line, _ := runtime.Caller(1)
	idx := strings.LastIndex(file, "/")
	prefix := fmt.Sprint("[Gographer warning @", file[idx + 1:], ":", line, "]")
	a = append([]interface{}{prefix}, a...)
	fmt.Println(a...)
	fmt.Printf("%s", debug.Stack())
}
