package data

import (
	gg "github.com/xinhuang327/gographer"
	"github.com/graphql-go/relay"
)

func init() {
	AddTodo("Hello Todo", false)
	AddTodo("Eat dinner", true)
	AddTodo("Sleep tight", false)
}

type Root struct{}

type Mutation struct{}

func (m *Mutation) AddTodo(text string) (todoEdge relay.EdgeType, viewer *User) {
	todoId := AddTodo(text, false)
	todo := GetTodo(todoId)
	// TODO: manage pagination
	return relay.EdgeType{
		Node:   todo,
		Cursor: relay.CursorForObjectInConnection(TodosToSliceInterface(GetTodos("any")), todo),
	}, GetViewer()
}

func (m *Mutation) ChangeTodoStatus(id string, complete bool) (todo *Todo, viewer *User) {
	resolvedId := relay.FromGlobalID(id) // TODO: ID conversion could be handled outside the function
	todoID := resolvedId.ID
	ChangeTodoStatus(todoID, complete)
	return GetTodo(todoID), GetViewer()
}

func (m *Mutation) MarkAllTodos(complete bool) (changedTodos []*Todo, viewer *User) {
	todoIds := MarkAllTodos(complete)
	todos := []*Todo{}
	for _, todoId := range todoIds {
		todo := GetTodo(todoId)
		if todo != nil {
			todos = append(todos, todo)
		}
	}
	return todos, GetViewer()
}

func (m *Mutation) RemoveCompletedTodos() (deletedTodoIds []string, viewer *User) {
	return RemoveCompletedTodos(), GetViewer()
}

func (m *Mutation) RemoveTodo(id string) (deletedTodoId string, viewer *User) {
	resolvedId := relay.FromGlobalID(id)
	RemoveTodo(resolvedId.ID)
	return relay.ToGlobalID(resolvedId.Type, resolvedId.ID), GetViewer()
}

func (m *Mutation) RenameTodo(id string, text string) (todo *Todo, viewer *User) {
	resolvedId := relay.FromGlobalID(id)
	todoID := resolvedId.ID
	RenameTodo(todoID, text)
	return GetTodo(todoID), GetViewer()
}

func (r *Root) GetViewer() *User {
	return GetViewer()
}

func (u *User) GetTodos(status string) []*Todo {
	return GetTodos(status)
}

func (u *User) GetTotalCount() int {
	return len(GetTodos("any"))
}

func (u *User) GetCompletedCount() int {
	return len(GetTodos("completed"))
}

func GetModelSchemaInfo() *gg.SchemaInfo {
	sch := gg.NewSchemaInfo()

	sch.RegType(Todo{}).
	SetIDResolver(func(id string) interface{} {
		return GetTodo(id)
	}).
	IDField("id", nil).
	SimpleField("text").
	SimpleField("complete")

	sch.RegType(User{}).
	SetIDResolver(func(id string) interface{} {
		return GetUser(id)
	}).
	IDField("id", nil).
	ResolvedField("todos", "GetTodos",
		gg.ArgInfo{"status", "any", false}).
	ResolvedField("totalCount", "GetTotalCount").
	ResolvedField("completedCount", "GetCompletedCount")

	sch.RegType(&Root{}).SetRoot().
	ResolvedField("viewer", "GetViewer")

	sch.RegType(&Mutation{}).SetMutation().
	MutationField("addTodo", "AddTodo",
		[]gg.ArgInfo{
			gg.ArgInfo{"text", nil, true},
		}, []gg.OutputInfo{
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