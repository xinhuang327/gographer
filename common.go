package gographer

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
	"fmt"
	"strconv"
	"runtime"
	"runtime/debug"
)

func getComplexQLType(
	returnType reflect.Type,
	fieldName string,
	qlTypes map[string]*graphql.Object,
	qlConns map[string]*relay.GraphQLConnectionDefinitions) (graphql.Output, QLTypeKind) {

	var returnQLType graphql.Output
	var qlTypeKind QLTypeKind

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

	var elemTypeName = elemType.Name()
	isPrimitive := true
	if elemQLType = ToQLType(elemType); elemQLType == nil {
		isPrimitive = false
		if qlType, ok := qlTypes[elemTypeName]; ok {
			elemQLType = qlType
		}
	}

	inferredElemTypeName := inferTypeNameFromField(fieldName)

	//
	//if elemType == reflect.TypeOf(relay.EdgeType{}) {
	//	if inferredElemTypeName != "" {
	//		elemQLType := qlTypes[inferredElemTypeName]
	//		conn := getOrCreateConnection(inferredElemTypeName, elemQLType, qlConns)
	//		returnQLType = conn.EdgeType
	//		qlTypeKind = QLTypeKind_Edge
	//	} else {
	//		Warning("Cannot infer type name for connection", fieldName, elemType)
	//	}
	//}

	// TODO: Better handling Connection and Edge types...
	if elemQLType != nil {
		if !isList {
			returnQLType = elemQLType
			if isPrimitive {
				qlTypeKind = QLTypeKind_Simple
			} else {
				qlTypeKind = QLTypeKind_Struct
			}
		} else {
			isConnection := false
			if isConnection {
				// connection
				conn := getOrCreateConnection(elemTypeName, elemQLType, qlConns)
				returnQLType = conn.ConnectionType
				qlTypeKind = QLTypeKind_Connection
			} else {
				// list
				returnQLType = graphql.NewList(elemQLType)
				qlTypeKind = QLTypeKind_SimpleList
			}
		}
	} else if elemType == reflect.TypeOf(relay.EdgeType{}) {
		if inferredElemTypeName != "" {
			elemQLType := qlTypes[inferredElemTypeName]
			conn := getOrCreateConnection(inferredElemTypeName, elemQLType, qlConns)
			returnQLType = conn.EdgeType
			qlTypeKind = QLTypeKind_Edge
		} else {
			Warning("Cannot infer type name for connection", fieldName, elemType)
		}
	} else {
		Warning("Cannot resolve QL type for return type", returnType, "elemType", elemType)
		fmt.Println(qlTypes)
	}

	return returnQLType, qlTypeKind
}

func getOrCreateConnection(
	elemTypeName string,
	elemQLType graphql.Output,
	qlConns map[string]*relay.GraphQLConnectionDefinitions) *relay.GraphQLConnectionDefinitions {
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

func inferTypeNameFromField(fieldName string) string {
	fieldName = upperFirst(fieldName)
	if strings.HasSuffix(fieldName, "Edge") {
		return strings.TrimSuffix(fieldName, "Edge")
	}
	if strings.HasSuffix(fieldName, "Connection") {
		return strings.TrimSuffix(fieldName, "Connection")
	}
	return ""
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
