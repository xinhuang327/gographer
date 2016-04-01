package data

import "github.com/xinhuang327/gographer"

func init() {
	AddTodo("Hello Todo", false)
	AddTodo("Eat dinner", true)
	AddTodo("Sleep tight", false)
}

type Root struct{}

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

func GetModelSchemaInfo() *gographer.SchemaInfo {
	sch := gographer.NewSchemaInfo()

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
	ResolvedField("todos", "GetTodos", "status", "any").
	ResolvedField("totalCount", "GetTotalCount").
	ResolvedField("completedCount", "GetCompletedCount")

	sch.RegType(&Root{}).SetRoot().
	ResolvedField("viewer", "GetViewer")

	return sch
}