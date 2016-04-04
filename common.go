package gographer

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
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

	elemTypeName := elemType.Name()
	isPrimitive := true
	if elemQLType = ToQLType(elemType); elemQLType == nil {
		isPrimitive = false
		if qlType, ok := qlTypes[elemTypeName]; ok {
			elemQLType = qlType
		}
	}

	inferredElemTypeName := inferTypeNameFromField(fieldName)

	if elemQLType != nil {
		if !isList {
			returnQLType = elemQLType
			if isPrimitive {
				qlTypeKind = QLTypeKind_Simple
			} else {
				qlTypeKind = QLTypeKind_Struct
			}
		} else {
			if isPrimitive {
				// primitive list
				returnQLType = graphql.NewList(elemQLType)
				qlTypeKind = QLTypeKind_SimpleList
			} else {
				// is connection
				conn := getOrCreateConnection(elemTypeName, elemQLType, qlConns)
				returnQLType = conn.ConnectionType
				qlTypeKind = QLTypeKind_Connection
			}
		}
	} else if elemType == reflect.TypeOf(relay.EdgeType{}) {
		if inferredElemTypeName != "" {
			conn := getOrCreateConnection(inferredElemTypeName, elemQLType, qlConns)
			returnQLType = conn.EdgeType
			qlTypeKind = QLTypeKind_Edge
		} else {
			Warning("Cannot infer type name for connection", fieldName, elemType)
		}
	} else {
		Warning("Cannot resolve QL type for return type", returnType)
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
