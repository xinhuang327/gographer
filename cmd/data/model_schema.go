package data

import (
	gg "github.com/xinhuang327/gographer"
)

//func init() {
//	AddTodo("Hello Todo", false)
//	AddTodo("Eat dinner", true)
//	AddTodo("Sleep tight", false)
//}

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
	IDField("id", nil).
	ResolvedField("todos", "GetTodos", gg.AutoArgs).
	ResolvedField("totalCount", "GetTotalCount", nil).
	ResolvedField("completedCount", "GetCompletedCount", nil)

	sch.RegType(&Root{}).SetRoot().
	ResolvedField("viewer", "GetViewer", nil)

	sch.RegType(&Mutation{}).SetMutation().
	MutationField("addTodo", "AddTodo", gg.AutoArgs, []gg.OutputInfo{
		gg.OutputInfo{"todoEdge", Todo{}}, // for edge type, need to specify element type for processor to find corresponding connection type
		gg.OutputInfo{"viewer", nil},
	}).
	MutationField("changeTodoStatus", "ChangeTodoStatus",
		[]gg.ArgInfo{
			gg.ArgInfo{"id", nil, true}, // TODO: specify type as ID?
			gg.ArgInfo{"complete", nil, true},
		}, []gg.OutputInfo{
			gg.OutputInfo{"todo", nil},
			gg.OutputInfo{"viewer", nil},
		}).
	MutationField("markAllTodos", "MarkAllTodos",
		[]gg.ArgInfo{
			gg.ArgInfo{"complete", nil, true},
		}, []gg.OutputInfo{
			gg.OutputInfo{"changedTodos", nil},
			gg.OutputInfo{"viewer", nil},
		}).
	MutationField("removeCompletedTodos", "RemoveCompletedTodos",
		[]gg.ArgInfo{
		}, []gg.OutputInfo{
			gg.OutputInfo{"deletedTodoIds", nil},
			gg.OutputInfo{"viewer", nil},
		}).
	MutationField("removeTodo", "RemoveTodo",
		[]gg.ArgInfo{
			gg.ArgInfo{"id", nil, true},
		}, []gg.OutputInfo{
			gg.OutputInfo{"deletedTodoId", nil},
			gg.OutputInfo{"viewer", nil},
		}).
	MutationField("renameTodo", "RenameTodo",
		[]gg.ArgInfo{
			gg.ArgInfo{"id", nil, true},
			gg.ArgInfo{"text", nil, true},
		}, []gg.OutputInfo{
			gg.OutputInfo{"todo", nil},
			gg.OutputInfo{"viewer", nil},
		})

	return sch
}
