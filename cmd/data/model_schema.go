package data

import (
	gg "github.com/xinhuang327/gographer"
)

func GetModelSchemaInfo() *gg.SchemaInfo {
	sch := gg.NewSchemaInfo()

	sch.RegType(Todo{}).
		SetIDResolver(func(id string) interface{} {
			return GetTodo(id)
		}).
		IDField("id", nil).SimpleFields()

	sch.RegType(User{}).
		SetIDResolver(func(id string) interface{} {
			return GetUser(id)
		}).
		IDField("id", nil).ResolvedFields()

	sch.RegType(&Root{}).SetRoot().ResolvedFields()

	sch.RegType(&Mutation{}).SetMutation().MutationFields()

	return sch
}
